package utils

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

var trustedProxies []string

func init() {
	proxies := os.Getenv("TRUSTED_PROXIES")
	if proxies != "" {
		trustedProxies = strings.Split(proxies, ",")
		for i := range trustedProxies {
			trustedProxies[i] = strings.TrimSpace(trustedProxies[i])
		}
	} else {
		trustedProxies = []string{"127.0.0.1", "::1", "localhost"}
	}
}

func isTrustedProxy(ip string) bool {
	for _, tp := range trustedProxies {
		if ip == tp {
			return true
		}
	}
	return false
}

func isPrivateIP(ip string) bool {
	cleanIP := strings.TrimPrefix(ip, "::ffff:")
	cleanIP = strings.Split(cleanIP, ":")[0]

	if strings.HasPrefix(cleanIP, "10.") ||
		strings.HasPrefix(cleanIP, "192.168.") {
		return true
	}
	if strings.HasPrefix(cleanIP, "172.") {
		if addr, err := netip.ParseAddr(cleanIP); err == nil {
			if prefix172 := netip.MustParsePrefix("172.16.0.0/12"); prefix172.Contains(addr) {
				return true
			}
		}
	}
	return false
}

// IsValidOrigin checks if the request origin matches the expected host.
func IsValidOrigin(origin, host string) bool {
	if origin == "" || host == "" {
		return true
	}
	// Remove port from host
	h := strings.Split(host, ":")[0]
	// Parse origin URL and compare hostname exactly (prevents substring match bypass)
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Hostname() == h
}

func isLocalhost(ip string) bool {
	cleanIP := strings.TrimPrefix(ip, "::ffff:")
	cleanIP = strings.Split(cleanIP, ":")[0]
	return cleanIP == "127.0.0.1" || cleanIP == "::1" || cleanIP == "localhost"
}

// IsLoopbackAddr reports whether the given "host:port" or "ip:port" remote
// address belongs to the loopback interface (127.0.0.1 / ::1). Used to decide
// whether a request genuinely originated from a trusted local proxy (e.g. the
// fnOS gateway unix-socket proxy) rather than a direct client connection.
func IsLoopbackAddr(remoteAddr string) bool {
	if remoteAddr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port present — treat the whole string as host.
		host = remoteAddr
	}
	cleanIP := strings.TrimPrefix(host, "::ffff:")
	cleanIP = strings.Split(cleanIP, ":")[0]
	return cleanIP == "127.0.0.1" || cleanIP == "::1" || cleanIP == "localhost"
}

func getFirstHeader(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// GetClientIP extracts the real client IP from the request.
func GetClientIP(r *http.Request) string {
	// Try to get from socket
	socketIP := ""
	if r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			socketIP = host
		} else {
			socketIP = r.RemoteAddr
		}
	}

	isDirect := r.Header.Get("X-Forwarded-For") == "" && r.Header.Get("X-Real-IP") == ""
	if isDirect {
		if socketIP == "" {
			return "unknown"
		}
		return socketIP
	}

	// Check X-Real-IP
	xRealIP := getFirstHeader(r.Header["X-Real-IP"])
	if xRealIP != "" && isTrustedProxy(socketIP) {
		cleanIP := strings.TrimPrefix(xRealIP, "::ffff:")
		cleanIP = strings.Split(cleanIP, ":")[0]
		if cleanIP != "" && !isLocalhost(cleanIP) {
			return cleanIP
		}
	}

	// Check CF-Connecting-IP
	cfIP := getFirstHeader(r.Header["Cf-Connecting-Ip"])
	if cfIP != "" && isTrustedProxy(socketIP) {
		cleanIP := strings.TrimPrefix(cfIP, "::ffff:")
		cleanIP = strings.Split(cleanIP, ":")[0]
		if cleanIP != "" && !isLocalhost(cleanIP) {
			return cleanIP
		}
	}

	// Check X-Forwarded-For
	if isTrustedProxy(socketIP) {
		xForwardedFor := getFirstHeader(r.Header["X-Forwarded-For"])
		if xForwardedFor != "" {
			ips := strings.Split(xForwardedFor, ",")
			for i := range ips {
				ips[i] = strings.TrimSpace(ips[i])
			}
			for _, ip := range ips {
				if ip != "" && !isLocalhost(ip) && !isPrivateIP(ip) {
					return ip
				}
			}
			if len(ips) > 0 && ips[0] != "" {
				return ips[0]
			}
		}
	}

	if socketIP == "" {
		return "unknown"
	}
	return socketIP
}

// IsPrivateURL checks if a URL points to a private/reserved IP address.
func IsPrivateURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true // reject unparseable URLs
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return true
	}

	// Strip IPv6 prefix
	if strings.HasPrefix(hostname, "::ffff:") {
		hostname = strings.TrimPrefix(hostname, "::ffff:")
	}

	// Check well-known loopback and private hostnames
	if hostname == "localhost" || hostname == "127.0.0.1" ||
		hostname == "::1" || hostname == "0.0.0.0" {
		return true
	}

	// Check protocol
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return true
	}

	// Check private IPv4 ranges
	if strings.HasPrefix(hostname, "192.168.") ||
		strings.HasPrefix(hostname, "10.") ||
		strings.HasPrefix(hostname, "169.254.") {
		return true
	}

	// Check 172.16.0.0/12
	if strings.HasPrefix(hostname, "172.") {
		if addr, err := netip.ParseAddr(hostname); err == nil {
			if prefix172 := netip.MustParsePrefix("172.16.0.0/12"); prefix172.Contains(addr) {
				return true
			}
		}
	}

	// Check IPv6 private ranges (fc00::/7, fd00::/7, fe80::/10)
	if strings.HasPrefix(hostname, "fc") ||
		strings.HasPrefix(hostname, "fd") ||
		strings.HasPrefix(hostname, "fe80") {
		return true
	}

	return false
}

// cleanIPStr strips the "::ffff:" IPv4-mapped IPv6 prefix and removes any port
// suffix, returning a clean IP address string suitable for parsing.
func cleanIPStr(ip string) string {
	s := strings.TrimPrefix(ip, "::ffff:")
	// Remove port if present (e.g. "192.168.1.1:8080" → "192.168.1.1")
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return s
}

// IsIPInCIDR checks whether the given IP address falls within the specified
// CIDR range. Supports both IPv4 and IPv6.
//
// The cidr parameter accepts:
//   - CIDR notation: "192.168.1.0/24", "10.0.0.0/8", "::1/128"
//   - Exact IP: "192.168.1.100" (treated as /32 or /128)
//
// Returns false if either the IP or the CIDR string is malformed.
func IsIPInCIDR(ip, cidr string) bool {
	addr, err := netip.ParseAddr(cleanIPStr(ip))
	if err != nil {
		return false
	}

	// If no '/' in cidr, treat as an exact IP match
	if !strings.Contains(cidr, "/") {
		other, err := netip.ParseAddr(cleanIPStr(cidr))
		if err != nil {
			return false
		}
		return addr.Compare(other) == 0
	}

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}
	return prefix.Contains(addr)
}

// IsIPInAnyCIDR checks whether the given IP address falls within any of the
// specified CIDR ranges. Returns false for an empty or nil slice.
func IsIPInAnyCIDR(ip string, cidrs []string) bool {
	for _, cidr := range cidrs {
		if IsIPInCIDR(ip, cidr) {
			return true
		}
	}
	return false
}

// IsPrivateIP exported wrapper for isPrivateIP, used by the notification IP
// whitelist feature to determine if an IP belongs to a private range.
func IsPrivateIP(ip string) bool {
	return isPrivateIP(ip)
}
