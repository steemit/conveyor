package usersearch

import (
	"strings"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

// Search holds the dependencies for the user-search RPC handlers.
type Search struct {
	client *CachingClient
	trie   *AccountNameTrie
}

// New creates a Search instance.
func New(client *CachingClient, trie *AccountNameTrie) *Search {
	return &Search{client: client, trie: trie}
}

// Register registers the user-search RPC methods (both public).
func (s *Search) Register(rpc *jsonrpc.Server) {
	rpc.Register("conveyor.get_account", s.getAccount)
	rpc.Register("conveyor.autocomplete_account", s.autocompleteAccount)
}

// getAccount returns account info for the given account name. GDPR accounts
// return an empty array []. contextAccount is optional.
func (s *Search) getAccount(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account        string `json:"account"`
		ContextAccount string `json:"context_account"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}

	// GDPR guard: return empty array.
	if _, gdpr := gdprList[p.Account]; gdpr {
		return []any{}, nil
	}

	j := s.client.LoadAccountJSON(p.Account, p.ContextAccount, 30)
	if j == nil {
		return []any{}, nil
	}
	return j, nil
}

// autocompleteResponse mirrors the TS AutoCompleteResponse.
type autocompleteResponse struct {
	Global  []*UserAccountJSON `json:"global"`
	Friends []*UserAccountJSON `json:"friends"`
	Recent  []*UserAccountJSON `json:"recent"`
}

// autocompleteAccount returns account suggestions matching a prefix.
// accountSubstring is the prefix; account is the requesting user.
func (s *Search) autocompleteAccount(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		AccountSubstring string `json:"account_substring"`
		Account          string `json:"account"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}

	// Load the requester's full account for friends/recent sets.
	userAccount, _ := s.client.LoadAccount(p.Account, "", 30)
	if userAccount == nil {
		return autocompleteResponse{}, nil
	}

	// Global matches from trie (only if substring > 3 chars).
	var globalNames []string
	if len(p.AccountSubstring) > 3 && s.trie != nil {
		globalNames = s.trie.MatchPrefix(p.AccountSubstring)
	}

	// Friends: filter following by prefix.
	var friendNames []string
	if userAccount.Following != nil {
		for name := range userAccount.Following {
			if strings.HasPrefix(name, p.AccountSubstring) {
				friendNames = append(friendNames, name)
			}
		}
	}

	// Recent: filter recent transfer targets by prefix.
	recentAccounts := userAccount.RecentSendAccounts()
	var recentNames []string
	for _, name := range recentAccounts {
		if strings.HasPrefix(name, p.AccountSubstring) {
			recentNames = append(recentNames, name)
		}
	}

	// Build the set of accounts to load — only if bucket has < 11 matches.
	accountsToLoadSet := make(map[string]struct{})
	if len(globalNames) < 11 {
		for _, n := range globalNames {
			accountsToLoadSet[n] = struct{}{}
		}
	}
	if len(friendNames) < 11 {
		for _, n := range friendNames {
			accountsToLoadSet[n] = struct{}{}
		}
	}
	if len(recentNames) < 11 {
		for _, n := range recentNames {
			accountsToLoadSet[n] = struct{}{}
		}
	}

	// Convert to slice and batch-load.
	var accountsToLoad []string
	for n := range accountsToLoadSet {
		accountsToLoad = append(accountsToLoad, n)
	}

	loaded := s.client.LoadAccountsJSON(accountsToLoad, p.Account, 30)

	// Partition loaded accounts into buckets.
	friendSet := make(map[string]struct{})
	for _, n := range friendNames {
		friendSet[n] = struct{}{}
	}
	recentSet := make(map[string]struct{})
	for _, n := range recentNames {
		recentSet[n] = struct{}{}
	}

	resp := autocompleteResponse{
		Global:  []*UserAccountJSON{},
		Friends: []*UserAccountJSON{},
		Recent:  []*UserAccountJSON{},
	}

	for _, acct := range loaded {
		if acct == nil {
			continue
		}
		name := acct.Account
		if _, ok := recentSet[name]; ok {
			resp.Recent = append(resp.Recent, acct)
		} else if _, ok := friendSet[name]; ok {
			resp.Friends = append(resp.Friends, acct)
		} else {
			resp.Global = append(resp.Global, acct)
		}
	}

	return resp, nil
}
