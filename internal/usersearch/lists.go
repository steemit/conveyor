// Package usersearch implements the account lookup and autocomplete subsystem,
// mirroring the original TS src/user-search/. It includes a CachingClient that
// fetches account data from steemd in parallel, a trie of all account names for
// prefix matching, and the get_account / autocomplete_account RPC handlers.
package usersearch

import (
	_ "embed"
	"regexp"
)

//go:embed embed_bad_actors.txt
var badActorsData string

//go:embed embed_exchanges.txt
var exchangesData string

//go:embed embed_gdpr.txt
var gdprData string

// User list sets, parsed from the embedded JS source at init time.
var (
	badActorsList = make(map[string]struct{})
	exchangesList = make(map[string]struct{})
	gdprList      = make(map[string]struct{})
)

// quotedStringRe extracts quoted strings from JS source (for parsing Set([...]) files).
var quotedStringRe = regexp.MustCompile(`['"]([^'"]+)['"]`)

func init() {
	parseList(badActorsData, badActorsList)
	parseList(exchangesData, exchangesList)
	parseList(gdprData, gdprList)
}

func parseList(src string, dst map[string]struct{}) {
	for _, m := range quotedStringRe.FindAllStringSubmatch(src, -1) {
		dst[m[1]] = struct{}{}
	}
}

// getUserTags returns the classification tags for an account, mirroring TS
// lists.ts getUserTags. Priority: exchange > abuse > verified(empty) > none.
func getUserTags(account string) []string {
	if _, ok := exchangesList[account]; ok {
		return []string{"exchange"}
	}
	if _, ok := badActorsList[account]; ok {
		return []string{"abuse"}
	}
	// verified list is always empty (FIXME in original TS).
	return []string{"none"}
}
