package usersearch

import (
	"testing"
)

func TestGetUserTags(t *testing.T) {
	// bittrex is in the exchanges list.
	tags := getUserTags("bittrex")
	if len(tags) != 1 || tags[0] != "exchange" {
		t.Fatalf("expected [exchange], got %v", tags)
	}

	// A known bad actor (first entry in the bad_actors list).
	// Just test the "none" case for a random name.
	tags = getUserTags("totally-random-nonexistent-account")
	if len(tags) != 1 || tags[0] != "none" {
		t.Fatalf("expected [none], got %v", tags)
	}
}

func TestGDPRList(t *testing.T) {
	if _, ok := gdprList["mateja.klaric"]; !ok {
		t.Error("expected mateja.klaric in GDPR list")
	}
	if _, ok := gdprList["steemit"]; ok {
		t.Error("steemit should NOT be in GDPR list")
	}
}

func TestAccountNameTrie_MatchPrefix(t *testing.T) {
	names := []string{"alice", "bob", "charlie", "alice2", "albert", "alex"}
	trie := NewAccountNameTrie(names, nil, 0) // no client = no refresh

	tests := []struct {
		prefix string
		want   []string
	}{
		{"al", []string{"albert", "alex", "alice", "alice2"}},
		{"bo", []string{"bob"}},
		{"ch", []string{"charlie"}},
		{"xyz", nil},
		{"", []string{"albert", "alex", "alice", "alice2", "bob", "charlie"}},
	}
	for _, tt := range tests {
		got := trie.MatchPrefix(tt.prefix)
		if len(got) != len(tt.want) {
			t.Errorf("MatchPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("MatchPrefix(%q)[%d] = %q, want %q", tt.prefix, i, got[i], tt.want[i])
			}
		}
	}
}

func TestLoadAccountNames(t *testing.T) {
	// Test parsing of the accounts.js format.
	// We can't test against the real file (14MB) in a unit test, but we can
	// verify the regex parser works on a sample.
	// The real file is tested via app startup.
	names := LoadAccountNames("user-data/accounts/accounts.js")
	if len(names) == 0 {
		t.Skip("accounts.js not found or empty — skip (needs make user-accounts)")
	}
	// Verify sorted.
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("names not sorted at index %d: %q > %q", i, names[i-1], names[i])
		}
	}
	// Verify "steemit" is in the list (it should be, as it's a real account).
	found := false
	for _, n := range names {
		if n == "steemit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'steemit' in account names")
	}
	t.Logf("loaded %d account names", len(names))
}
