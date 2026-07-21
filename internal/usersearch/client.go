package usersearch

import (
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	steemapi "github.com/steemit/steemgosdk/api"
	protocolapi "github.com/steemit/steemutil/protocol/api"
)

const pageSize = 1000

// CachingClient wraps steemgosdk's API with a TTL cache for account data.
// It mirrors TS src/user-search/client.ts CachingClient.
type CachingClient struct {
	api   *steemapi.API
	cache *cache.Cache
}

// NewCachingClient creates a client backed by the given Steem RPC node.
// Cache TTL defaults to 600s (matching TS config cacheClient.ttl).
func NewCachingClient(rpcNode string, ttl, cleanupInterval time.Duration) *CachingClient {
	return &CachingClient{
		api:   steemapi.NewAPI(rpcNode),
		cache: cache.New(ttl, cleanupInterval),
	}
}

// --- Steemd fetch primitives ---

func (c *CachingClient) getExtendedAccount(account string) (*protocolapi.ExtendedAccount, error) {
	accts, err := c.api.GetAccounts([]string{account})
	if err != nil {
		return nil, err
	}
	if len(accts) == 0 {
		return nil, fmt.Errorf("account not found: %s", account)
	}
	return accts[0], nil
}

func (c *CachingClient) getFollowCount(account string) (*protocolapi.FollowCountReturn, error) {
	return c.api.GetFollowCount(account)
}

// getFollowers paginates through all followers of the given type ("blog" or "ignore").
func (c *CachingClient) getFollowers(account, followType string) ([]*protocolapi.FollowReturn, error) {
	var all []*protocolapi.FollowReturn
	start := ""
	for {
		page, err := c.api.GetFollowers(account, start, followType, pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		last := page[len(page)-1].Follower
		if last == start || len(page) < pageSize {
			break
		}
		start = last
	}
	return all, nil
}

// getFollowing paginates through all following.
func (c *CachingClient) getFollowing(account string) ([]*protocolapi.FollowReturn, error) {
	var all []*protocolapi.FollowReturn
	start := ""
	for {
		page, err := c.api.GetFollowing(account, start, "blog", pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		last := page[len(page)-1].Following
		if last == start || len(page) < pageSize {
			break
		}
		start = last
	}
	return all, nil
}

// getAccountTransferTargetCounts walks account history (last `days` days) and
// counts transfer recipients, mirroring TS getAccountTransferTargetCounts.
// Pagination follows the TS pattern: first call with from=-1 gets the newest
// page; the last entry's index tells us the total length; subsequent pages
// use absolute indices (totalLength, totalLength-pageSize, ...) walking
// backward toward older entries.
func (c *CachingClient) getAccountTransferTargetCounts(account string, days int) (map[string]int, error) {
	counts := make(map[string]int)
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	// First page: from=-1 fetches the most recent entries.
	history, err := c.api.GetAccountHistory(account, -1, pageSize)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return counts, nil
	}

	// Determine total history length from the last entry's index.
	lastIdx := history[len(history)-1].Index
	totalPages := int(lastIdx/pageSize) + 1

	// Process pages from newest to oldest.
	for pageNum := 0; pageNum < totalPages; pageNum++ {
		var page []*steemapi.AccountHistoryEntry
		if pageNum == 0 {
			page = history // reuse first-page result
		} else {
			// Compute the absolute index for this page (walk backward).
			pointer := lastIdx - int64(pageNum*pageSize)
			if pointer < 0 {
				pointer = 0
			}
			page, err = c.api.GetAccountHistory(account, pointer, pageSize)
			if err != nil {
				return nil, err
			}
			if len(page) == 0 {
				break
			}
		}

		// Walk entries newest-first within the page.
		for i := len(page) - 1; i >= 0; i-- {
			entry := page[i]
			ts, err := time.Parse("2006-01-02T15:04:05", entry.Timestamp)
			if err != nil {
				continue
			}
			if ts.Before(cutoff) {
				return counts, nil // older than cutoff, done
			}
			if entry.Op.Type == "transfer" {
				to, _ := entry.Op.Payload["to"].(string)
				if to != "" {
					counts[to]++
				}
			}
		}
	}

	return counts, nil
}

// --- Orchestration ---

// loadAccountInfo fetches all 6 data sources in parallel, mirroring TS Promise.all.
func (c *CachingClient) loadAccountInfo(account string, days int) (*UserAccount, error) {
	type result struct {
		ext       *protocolapi.ExtendedAccount
		fc        *protocolapi.FollowCountReturn
		transfers map[string]int
		followers []*protocolapi.FollowReturn
		following []*protocolapi.FollowReturn
		ignored   []*protocolapi.FollowReturn
	}

	var r result
	var extErr, fcErr, transferErr, followersErr, followingErr, ignoredErr error
	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		r.ext, extErr = c.getExtendedAccount(account)
	}()
	go func() {
		defer wg.Done()
		r.fc, fcErr = c.getFollowCount(account)
	}()
	go func() {
		defer wg.Done()
		r.transfers, transferErr = c.getAccountTransferTargetCounts(account, days)
	}()
	go func() {
		defer wg.Done()
		r.followers, followersErr = c.getFollowers(account, "blog")
	}()
	go func() {
		defer wg.Done()
		r.following, followingErr = c.getFollowing(account)
	}()
	go func() {
		defer wg.Done()
		r.ignored, ignoredErr = c.getFollowers(account, "ignore")
	}()

	wg.Wait()

	// Check errors — each goroutine writes its own error variable (no data race).
	for _, err := range []error{extErr, fcErr, transferErr, followersErr, followingErr, ignoredErr} {
		if err != nil {
			return nil, err
		}
	}

	return NewUserAccount(r.ext, r.fc, r.transfers, r.followers, r.following, r.ignored), nil
}

// accountCacheKey returns the cache key for a UserAccount.
func accountCacheKey(account string) string {
	return "UserAccount__" + account
}

// contextCacheKey returns the cache key for a UserContext.
func contextCacheKey(account, contextAccount string) string {
	return fmt.Sprintf("UserContext__%s__%s", account, contextAccount)
}

// LoadAccount loads a UserAccount (and optionally UserContext), with caching.
// GDPR accounts return (nil, nil).
func (c *CachingClient) LoadAccount(account string, contextAccount string, days int) (*UserAccount, *UserContext) {
	if _, gdpr := gdprList[account]; gdpr {
		return nil, nil
	}

	// Check cache for UserAccount.
	var ua *UserAccount
	if cached, ok := c.cache.Get(accountCacheKey(account)); ok {
		ua = cached.(*UserAccount)
	} else {
		loaded, err := c.loadAccountInfo(account, days)
		if err != nil || loaded == nil {
			return nil, nil
		}
		ua = loaded
		c.cache.SetDefault(accountCacheKey(account), ua)
	}

	if contextAccount == "" {
		return ua, nil
	}

	// Check cache for UserContext.
	ck := contextCacheKey(account, contextAccount)
	var uc *UserContext
	if cached, ok := c.cache.Get(ck); ok {
		uc = cached.(*UserContext)
	} else {
		uc = ua.UserContextFor(contextAccount)
		c.cache.SetDefault(ck, uc)
	}
	return ua, uc
}

// LoadAccountJSON returns the JSON representation of an account (with optional
// context). Returns nil for GDPR or not-found accounts.
func (c *CachingClient) LoadAccountJSON(account string, contextAccount string, days int) *UserAccountJSON {
	ua, uc := c.LoadAccount(account, contextAccount, days)
	if ua == nil {
		return nil
	}
	j := ua.ToJSONWithContext(uc)
	return &j
}

// LoadAccountsJSON batch-loads multiple accounts' JSON (with context).
func (c *CachingClient) LoadAccountsJSON(accounts []string, contextAccount string, days int) []*UserAccountJSON {
	results := make([]*UserAccountJSON, len(accounts))
	var wg sync.WaitGroup
	for i, acct := range accounts {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			results[idx] = c.LoadAccountJSON(name, contextAccount, days)
		}(i, acct)
	}
	wg.Wait()
	return results
}
