package usersearch

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	steemapi "github.com/steemit/steemgosdk/api"
)

// quoteRe extracts quoted strings from JS source (for parsing accounts.js).
var quoteRe = regexp.MustCompile(`"([^"]+)"`)

// LoadAccountNames reads the pre-built accounts.js file (a JS Set of all
// Steem account names) and returns them as a sorted slice. This is the
// initial trie data source — the TS version uses require() on the same file.
func LoadAccountNames(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	names := quoteRe.FindAllStringSubmatch(string(data), -1)
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	var result []string
	for _, m := range names {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		result = append(result, m[1])
	}
	sort.Strings(result)
	return result
}

// AccountNameTrie provides prefix matching over all Steem account names.
// It uses a sorted slice + binary search for prefix matching (simpler and
// fast enough for ~140k names). Mirrors TS AccountNameTrie.
type AccountNameTrie struct {
	mu          sync.RWMutex
	names       []string // sorted
	client      *CachingClient
	stopCh      chan struct{}
	ticker      *time.Ticker
	refreshInterval time.Duration
}

// NewAccountNameTrie creates a trie from the initial account name set.
func NewAccountNameTrie(initial []string, client *CachingClient, refreshInterval time.Duration) *AccountNameTrie {
	sort.Strings(initial)
	return &AccountNameTrie{
		names:           initial,
		client:          client,
		stopCh:          make(chan struct{}),
		refreshInterval: refreshInterval,
	}
}

// MatchPrefix returns all account names starting with the given prefix,
// up to a reasonable limit. Only meaningful for prefixes longer than 3 chars
// (matching TS autocompleteAccount's threshold).
func (t *AccountNameTrie) MatchPrefix(prefix string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Binary search for the first name >= prefix.
	left := sort.SearchStrings(t.names, prefix)
	var result []string
	for i := left; i < len(t.names); i++ {
		if !strings.HasPrefix(t.names[i], prefix) {
			break
		}
		result = append(result, t.names[i])
		if len(result) >= 100 {
			break // cap to avoid huge responses
		}
	}
	return result
}

// addNames incrementally adds new names to the trie (sorted insert).
func (t *AccountNameTrie) addNames(newNames []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	existing := make(map[string]struct{}, len(t.names))
	for _, n := range t.names {
		existing[n] = struct{}{}
	}
	for _, n := range newNames {
		if _, ok := existing[n]; !ok {
			existing[n] = struct{}{}
			t.names = append(t.names, n)
		}
	}
	sort.Strings(t.names)
}

// StartRefreshing begins periodic background refresh of the trie via
// lookup_accounts. Blocks on a goroutine; call StopRefreshing to halt.
func (t *AccountNameTrie) StartRefreshing() {
	t.ticker = time.NewTicker(t.refreshInterval)
	go func() {
		// Do an immediate refresh on start.
		t.updateTrie()
		for {
			select {
			case <-t.ticker.C:
				t.updateTrie()
			case <-t.stopCh:
				return
			}
		}
	}()
}

// StopRefreshing halts the background refresh loop.
func (t *AccountNameTrie) StopRefreshing() {
	if t.ticker != nil {
		t.ticker.Stop()
	}
	select {
	case <-t.stopCh: // already closed
	default:
		close(t.stopCh)
	}
}

// updateTrie walks lookup_accounts from '' and incrementally adds new names.
func (t *AccountNameTrie) updateTrie() {
	if t.client == nil {
		return
	}
	start := ""
	for {
		page, err := t.client.api.LookupAccounts(start, pageSize)
		if err != nil {
			break
		}
		if len(page) == 0 {
			break
		}
		t.addNames(page)
		last := page[len(page)-1]
		if last == start || len(page) < pageSize {
			break
		}
		start = last
	}
}

// loadAllAccountNames loads ALL account names via 27 parallel shards
// (a/b/c.../z), mirroring TS loadAllAccountNames. Used by the offline
// account loader script; not called during normal server operation.
func LoadAllAccountNames(api *steemapi.API) []string {
	starts := append([]string{""}, letters("bcdefghijklmnopqrstuvwxyz")...)
	ends := starts[1:] // one shorter; last shard has end=""

	type shardResult struct {
		names []string
		err   error
	}
	results := make([]shardResult, len(starts))
	var wg sync.WaitGroup

	for i, start := range starts {
		wg.Add(1)
		go func(idx int, s, e string) {
			defer wg.Done()
			names, err := loadAccountNamesShard(api, s, e)
			results[idx] = shardResult{names: names, err: err}
		}(i, start, "")
		_ = ends // ends bounds checking is done inside loadAccountNamesShard
	}
	wg.Wait()

	seen := make(map[string]struct{})
	var all []string
	for _, r := range results {
		if r.err != nil {
			continue
		}
		for _, n := range r.names {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				all = append(all, n)
			}
		}
	}
	sort.Strings(all)
	return all
}

func loadAccountNamesShard(api *steemapi.API, start, end string) ([]string, error) {
	var names []string
	cursor := start
	for {
		page, err := api.LookupAccounts(cursor, pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		last := page[len(page)-1]
		if last == cursor || len(page) < pageSize {
			names = append(names, page...)
			break
		}
		if end != "" && strings.HasPrefix(last, end) {
			// Reached the next shard's prefix; filter out names starting with end.
			for _, n := range page {
				if !strings.HasPrefix(n, end) {
					names = append(names, n)
				}
			}
			break
		}
		names = append(names, page...)
		cursor = last
	}
	return names, nil
}

func letters(s string) []string {
	var result []string
	for _, r := range s {
		result = append(result, string(r))
	}
	return result
}
