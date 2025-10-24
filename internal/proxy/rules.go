package proxy

import (
	"strings"
	"time"
)

type Rules struct {
	passthrough      []string
	httpsPassthrough []string
	specialTTL       map[string]time.Duration
}

func NewRules(passthrough []string, httpsPassthrough []string, specialTTL map[string]string) (*Rules, error) {
	ttlMap := make(map[string]time.Duration)
	for pattern, ttlStr := range specialTTL {
		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			return nil, err
		}
		ttlMap[pattern] = ttl
	}

	return &Rules{
		passthrough:      passthrough,
		httpsPassthrough: httpsPassthrough,
		specialTTL:       ttlMap,
	}, nil
}

func (r *Rules) ShouldPassthrough(url string) bool {
	for _, pattern := range r.passthrough {
		if matchPattern(url, pattern) {
			return true
		}
	}
	return false
}

func (r *Rules) ShouldAllowHTTPS(host string) bool {
	for _, pattern := range r.httpsPassthrough {
		if matchPattern(host, pattern) {
			return true
		}
	}
	return false
}

func (r *Rules) GetTTL(url string, defaultTTL time.Duration) time.Duration {
	if strings.Contains(url, "InRelease") || strings.Contains(url, "Release.gpg") {
		return 5 * time.Minute
	}

	if strings.Contains(url, "/Release") && !strings.Contains(url, "InRelease") {
		return 30 * time.Minute
	}

	if strings.Contains(url, "/Packages") || strings.Contains(url, "/Sources") {
		return 1 * time.Hour
	}

	for pattern, ttl := range r.specialTTL {
		if matchPattern(url, pattern) {
			return ttl
		}
	}

	return defaultTTL
}

func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(s, pattern[1:len(pattern)-1])
	}

	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}

	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}

	return strings.Contains(s, pattern)
}
