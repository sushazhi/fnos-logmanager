package utils

import (
	"net"
	"net/http"
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
		parts := strings.Split(cleanIP, ".")
		if len(parts) >= 2 {
			second := parts[1]
			if len(second) == 2 || (len(second) == 1 && second >= "16" && second <= "31") ||
				(len(second) == 2 && second >= "16" && second <= "31") {
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
		parts := strings.SplitN(hostname, ".", 3)
		if len(parts) >= 2 {
			if second := parts[1]; len(second) > 0 {
				if second[0] >= '1' && second[0] <= '3' {
					return true
				}
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
