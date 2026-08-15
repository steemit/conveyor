package usersearch

import (
	"math/rand"
	"sort"
	"testing"
)

func TestMergeSortedNames_Basic(t *testing.T) {
	a := []string{"alice", "bob", "dave"}
	b := []string{"bob", "carol", "eve"}
	got := mergeSortedNames(a, b)
	want := []string{"alice", "bob", "carol", "dave", "eve"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestMergeSortedNames_EmptyInputs(t *testing.T) {
	a := []string{"alice", "bob"}
	if got := mergeSortedNames(a, nil); len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("merge with empty b: %v", got)
	}
	if got := mergeSortedNames(nil, a); len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("merge with empty a: %v", got)
	}
	if got := mergeSortedNames(nil, nil); len(got) != 0 {
		t.Fatalf("merge empty+empty: %v", got)
	}
}

func TestMergeSortedNames_TotalOverlap(t *testing.T) {
	a := []string{"a", "b", "c"}
	got := mergeSortedNames(a, []string{"a", "b", "c"})
	if len(got) != 3 {
		t.Fatalf("expected dedup to 3, got %d: %v", len(got), got)
	}
}

func TestMergeSortedNames_RandomizedAgainstNaive(t *testing.T) {
	// Property test: the merge must equal map-dedup + sort for arbitrary
	// sorted inputs with duplicates.
	rng := rand.New(rand.NewSource(42))
	for round := 0; round < 100; round++ {
		a := randomSortedNames(rng, rng.Intn(200))
		b := randomSortedNames(rng, rng.Intn(200))

		got := mergeSortedNames(a, b)

		// Naive reference: map dedup + sort.
		set := map[string]struct{}{}
		for _, n := range append(append([]string{}, a...), b...) {
			set[n] = struct{}{}
		}
		want := make([]string, 0, len(set))
		for n := range set {
			want = append(want, n)
		}
		sort.Strings(want)

		if len(got) != len(want) {
			t.Fatalf("round %d: len got %d want %d", round, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("round %d: index %d got %q want %q", round, i, got[i], want[i])
			}
		}
		if !isStrictlySorted(got) {
			t.Fatalf("round %d: result not strictly sorted", round)
		}
	}
}

func randomSortedNames(rng *rand.Rand, n int) []string {
	names := make([]string, n)
	for i := range names {
		// Short alphabet → many collisions/duplicates across the two lists.
		names[i] = string(rune('a' + rng.Intn(6))) + string(rune('a' + rng.Intn(6))) + string(rune('a' + rng.Intn(6)))
	}
	sort.Strings(names)
	return names
}

func TestIsStrictlySorted(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{[]string{"a", "b", "c"}, true},
		{[]string{"a", "a"}, false}, // duplicate → not strict
		{[]string{"b", "a"}, false},
		{nil, true},
		{[]string{"only"}, true},
	}
	for _, c := range cases {
		if got := isStrictlySorted(c.in); got != c.want {
			t.Errorf("isStrictlySorted(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAccountNameTrie_AddNamesKeepsSortedInvariant(t *testing.T) {
	trie := NewAccountNameTrie([]string{"alice", "carol"}, nil, 0)
	trie.addNames([]string{"bob", "carol", "dave"})
	want := []string{"alice", "bob", "carol", "dave"}
	if len(trie.names) != len(want) {
		t.Fatalf("len: got %d want %d", len(trie.names), len(want))
	}
	for i := range want {
		if trie.names[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, trie.names[i], want[i])
		}
	}
	// Prefix search must still work on the merged trie.
	if got := trie.MatchPrefix("c"); len(got) != 1 || got[0] != "carol" {
		t.Fatalf("MatchPrefix(c) = %v", got)
	}
}
