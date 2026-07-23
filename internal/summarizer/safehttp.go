// This file implements the SSRF defenses for conveyor.summarize_url.
//
// summarize_url is a *public* (unauthenticated) endpoint that fetches an
// attacker-supplied URL. Without the controls here, an attacker could make
// conveyor issue requests to internal addresses (loopback, link-local such as
// the AWS IMDS endpoint 169.254.169.254, RFC1918 ranges) — either to read
// internal/metadata services or to use conveyor as a fetch amplifier.
//
// Defenses:
//   - validateScheme: only http/https (entrypoint and each redirect hop).
//   - safeDialContext: resolves the host itself, validates EVERY resolved IP
//     before connecting, and connects to the already-validated IP literal. No
//     second DNS lookup occurs, so a DNS rebinding attack (valid IP at check
//     time, private IP at connect time) cannot succeed.
//   - newSafeTransport wires the dialer into http.Transport.
package summarizer

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// errBlockedScheme is returned for non-http(s) URLs (including file://, which
// could otherwise read local files via goquery in some configurations).
var errBlockedScheme = errors.New("blocked URL scheme: only http and https are allowed")

// validateScheme rejects any scheme other than http/https. It is applied at the
// entrypoint and on every redirect hop.
func validateScheme(u *url.URL) error {
	if u == nil {
		return errors.New("nil URL")
	}
	s := strings.ToLower(u.Scheme)
	if s != "http" && s != "https" {
		return errBlockedScheme
	}
	return nil
}

// isBlockedIP reports whether the IP is in a range that must not be reached
// from an unauthenticated URL-fetch endpoint. This covers:
//   - loopback (127.0.0.0/8, ::1)
//   - private / RFC1918 (10/8, 172.16/12, 192.168/16, fc00::/7)
//   - link-local (169.254/16 incl. AWS IMDS, fe80::/10)
//   - unspecified (0.0.0.0, ::)
//   - multicast (224/4, ff00::/8)
//
// IPv4-in-IPv6 mapped addresses (::ffff:127.0.0.1) are normalized via To4() so
// they are caught by the IPv4 checks.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true // treat unparseable as blocked
	}
	// Normalize mapped IPv6 -> IPv4 so ::ffff:127.0.0.1 matches loopback.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return false
}

// safeDialContext is the SSRF-resistant dialer. It splits host:port, resolves
// the host itself, validates all resolved IPs, and then dials the first valid
// IP literal directly. The standard net.Dialer would re-resolve the hostname,
// opening a DNS-rebinding window between our check and the actual connect.
//
// We connect to the IP (not the hostname) and rely on the default behavior for
// Host/SNI: HTTP clients passing an IP-based authority preserve TLS SNI from
// the request's Host header. For the common (hostname) case this is correct
// because the transport dials the IP we validated.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := resolveAndValidate(ctx, host)
	if err != nil {
		return nil, err
	}

	// Dial the first validated IP. We pass an IP literal so no further DNS
	// resolution happens. This is the core rebinding defense.
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// resolveAndValidate resolves host and returns the set of IPs, but only if
// EVERY one of them passes isBlockedIP. If host is already a literal IP it is
// validated directly without any DNS lookup.
func resolveAndValidate(ctx context.Context, host string) ([]net.IP, error) {
	// Literal IP — validate directly, no DNS.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return nil, errors.New("blocked destination IP: " + host)
		}
		return []net.IP{ip}, nil
	}

	// Hostname — resolve and require ALL addresses to be non-internal. Allowing
	// even one private address would let an attacker publish a DNS record that
	// mixes public + private to slip through a "first record" check.
	resolver := net.DefaultResolver
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("no IP addresses resolved for " + host)
	}
	out := make([]net.IP, 0, len(ips))
	for _, ia := range ips {
		if isBlockedIP(ia.IP) {
			return nil, errors.New("blocked destination IP for " + host + ": " + ia.IP.String())
		}
		out = append(out, ia.IP)
	}
	return out, nil
}

// maxRedirects caps redirect chains. The http.Client default is 10; we make it
// explicit and re-validate the scheme of every hop via CheckRedirect.
const maxRedirects = 10

// newSafeTransport returns an http.RoundTripper whose dialer resolves and
// validates destination IPs (blocking loopback / private / link-local / etc.)
// and connects to a pre-validated IP literal so DNS rebinding cannot occur.
func newSafeTransport() http.RoundTripper {
	return &http.Transport{
		DialContext:     safeDialContext,
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: nil,
	}
}
