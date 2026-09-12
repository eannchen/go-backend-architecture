package observability

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

func parseServerEndpoint(target string) (string, int) {
	if target == "" {
		return "", 0
	}
	candidate := target
	if parsed, err := url.Parse(target); err == nil && (strings.Contains(target, "://") || strings.HasPrefix(target, "unix:")) {
		switch parsed.Scheme {
		case "dns":
			candidate = strings.TrimPrefix(parsed.Path, "/")
		case "unix":
			if parsed.Path != "" {
				return parsed.Path, 0
			}
		default:
			return target, 0
		}
	}
	host, portText, err := net.SplitHostPort(candidate)
	if err != nil {
		return target, 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return target, 0
	}
	return host, port
}
