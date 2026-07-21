package summarizer

// badDomains are domain suffixes to flag as blacklisted. A URL whose host
// ends with any of these is marked blacklisted=true (but still fetched).
// Mirrors TS blacklist.ts.
var badDomains = []string{
	"bad.domain",
}

// badUrls are host+path prefixes (no protocol) to flag as blacklisted.
// A URL whose host+pathname starts with any of these is marked blacklisted.
// Mirrors TS blacklist.ts.
var badUrls = []string{
	"bad.url/leave/out/protocol",
	"www.google.com",
}

// isBlacklisted checks whether the given host and path match the blacklist.
func isBlacklisted(host, path string) bool {
	// Domain suffix check.
	for _, d := range badDomains {
		if suffixMatch(host, d) {
			return true
		}
	}
	// URL prefix check (host + path, no protocol).
	urlLessProtocol := host + path
	for _, u := range badUrls {
		if len(urlLessProtocol) >= len(u) && urlLessProtocol[:len(u)] == u {
			return true
		}
	}
	return false
}

// suffixMatch checks if host ends with domain, mirroring JS str.endsWith().
func suffixMatch(host, domain string) bool {
	if len(host) < len(domain) {
		return false
	}
	return host[len(host)-len(domain):] == domain
}
