package usersearch

import (
	"context"
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

func (c *CachingClient) getExtendedAccount(ctx context.Context, account string) (*protocolapi.ExtendedAccount, error) {
	accts, err := c.api.GetAccounts([]string{account})
	if err != nil {
		return nil, err
	}
	if len(accts) == 0 {
		return nil, fmt.Errorf("account not found: %s", account)
	}
	return accts[0], nil
}

func (c *CachingClient) getFollowCount(ctx context.Context, account string) (*protocolapi.FollowCountReturn, error) {
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

// getAccountTransferTargetCounts walks account history (last 30 days) and
// counts transfer recipients, mirroring TS getAccountTransferTargetCounts.
func (c *CachingClient) getAccountTransferTargetCounts(account string, days int) (map[string]int, error) {
	counts := make(map[string]int)
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	pointer := int64(-1)
	totalPages := 1
	pageNum := 0

	for pageNum < totalPages {
		history, err := c.api.GetAccountHistory(account, pointer, pageSize)
		if err != nil {
			return nil, err
		}
		if len(history) == 0 {
			break
		}

		// On first page, determine total length from the last entry's index.
		if pageNum == 0 {
			lastIdx := history[len(history)-1].Index
			totalPages = int(lastIdx/pageSize) + 1
		}

		// Walk newest-first (history is returned oldest-first per page, but
		// the overall set spans from pointer downward).
		for i := len(history) - 1; i >= 0; i-- {
			entry := history[i]
			// Check timestamp cutoff.
			ts, err := time.Parse("2006-01-02T15:04:05", entry.Timestamp)
			if err != nil {
				continue
			}
			if ts.Before(cutoff) {
				return counts, nil // older than cutoff, done
			}
			// Count transfer operations.
			if entry.Op.Type == "transfer" {
				to, _ := entry.Op.Payload["to"].(string)
				if to != "" {
					counts[to]++
				}
			}
		}

		pageNum++
		if pageNum >= totalPages {
			break
		}
		pointer -= pageSize
		if pointer < pageSize {
			pointer = pageSize
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
		err       error
	}

	var r result
	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		r.ext, r.err = c.getExtendedAccount(context.Background(), account)
		if r.err != nil {
			return
		}
	}()
	go func() {
		defer wg.Done()
		r.fc, r.err = c.getFollowCount(context.Background(), account)
	}()

	// Transfer targets (may set r.err, but the others run independently).
	var transferErr error
	go func() {
		defer wg.Done()
		r.transfers, transferErr = c.getAccountTransferTargetCounts(account, days)
	}()

	var followersErr, followingErr, ignoredErr error
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

	// Check errors.
	if r.err != nil {
		return nil, r.err
	}
	if transferErr != nil {
		return nil, transferErr
	}
	if followersErr != nil {
		return nil, followersErr
	}
	if followingErr != nil {
		return nil, followingErr
	}
	if ignoredErr != nil {
		return nil, ignoredErr
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
