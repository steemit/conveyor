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

// addNames merges a sorted list of names into the trie. Both t.names and
// newNames must be sorted ascending; the merge deduplicates in a single
// O(N+M) pass (no map rebuild, no re-sort).
//
// Previously this rebuilt a map of ALL existing names and re-sorted the whole
// slice on every call — and updateTrie called it once PER PAGE (~1160 pages
// for ~1.16M accounts), costing ~34GB of cumulative allocations per refresh
// and pinning the CPU on small instances (observed 97% on t2.micro).
func (t *AccountNameTrie) addNames(newNames []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.names = mergeSortedNames(t.names, newNames)
}

// mergeSortedNames merges two ascending-sorted name lists into a new
// strictly-sorted (duplicate-free) slice in a single O(N+M) pass. Inputs must
// be sorted but MAY contain adjacent duplicates — the merge dedups both
// across lists and within each list. Callers guard the sorted invariant
// (see isStrictlySorted).
func mergeSortedNames(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	appendUnique := func(s string) {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			appendUnique(a[i])
			i++
		case a[i] > b[j]:
			appendUnique(b[j])
			j++
		default: // equal: keep one, advance both
			appendUnique(a[i])
			i++
			j++
		}
	}
	for ; i < len(a); i++ {
		appendUnique(a[i])
	}
	for ; j < len(b); j++ {
		appendUnique(b[j])
	}
	return out
}

// isStrictlySorted reports whether s is strictly ascending (no duplicates).
func isStrictlySorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] >= s[i] {
			return false
		}
	}
	return true
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

// updateTrie walks lookup_accounts from '' and merges the full account list
// into the trie.
//
// It collects ALL pages first (network I/O, no lock held — MatchPrefix stays
// responsive during the multi-minute walk) and then performs a single sorted
// merge. Pages from lookup_accounts are lexicographically ordered and the
// cursor concatenation preserves global order; each page is still checked
// (and defensively sorted) so an upstream behavior change can never corrupt
// the sorted invariant MatchPrefix's binary search depends on — worst case it
// degrades to a per-page sort, never to silently-wrong autocomplete results.
func (t *AccountNameTrie) updateTrie() {
	if t.client == nil {
		return
	}
	var collected []string
	start := ""
	for {
		page, err := t.client.api.LookupAccounts(start, pageSize)
		if err != nil {
			break
		}
		if len(page) == 0 {
			break
		}
		if !isStrictlySorted(page) {
			sort.Strings(page)
		}
		collected = append(collected, page...)
		last := page[len(page)-1]
		if last == start || len(page) < pageSize {
			break
		}
		start = last
	}
	if len(collected) == 0 {
		return
	}
	// Cross-page boundary sanity: concatenated pages must stay ascending.
	// If not (unexpected upstream behavior), sort the whole collection so the
	// merge precondition holds.
	if !isStrictlySorted(collected) {
		sort.Strings(collected)
	}
	t.addNames(collected)
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
		end := ""
		if i < len(ends) {
			end = ends[i]
		}
		go func(idx int, s, e string) {
			defer wg.Done()
			names, err := loadAccountNamesShard(api, s, e)
			results[idx] = shardResult{names: names, err: err}
		}(i, start, end)
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
