package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"cascade/internal/cache"
	"cascade/internal/config"
)

type Proxy struct {
	config    *config.Config
	storage   *cache.Storage
	transport *http.Transport
	rules     *Rules
	client    *http.Client
}

func New(cfg *config.Config, storage *cache.Storage) (*Proxy, error) {
	egressDialer, err := NewEgressDialer(
		cfg.Egress.ProxyType,
		cfg.Egress.ProxyURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create egress dialer: %w", err)
	}

	rules, err := NewRules(cfg.Rules.Passthrough, cfg.Rules.HTTPSPassthrough, cfg.Rules.SpecialTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create rules: %w", err)
	}

	transport := egressDialer.GetTransport()
	transport.MaxIdleConns = 1000
	transport.MaxIdleConnsPerHost = 100
	transport.MaxConnsPerHost = 100
	transport.IdleConnTimeout = 90 * time.Second
	transport.DisableCompression = false
	transport.ForceAttemptHTTP2 = false

	return &Proxy{
		config:    cfg,
		storage:   storage,
		transport: transport,
		rules:     rules,
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Minute,
		},
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}

	targetURL := r.URL.String()
	if targetURL == "" || (!strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://")) {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		p.forwardRequest(w, r, targetURL)
		return
	}

	if p.rules.ShouldPassthrough(targetURL) {
		log.Printf("[PASSTHROUGH] %s", targetURL)
		p.forwardRequest(w, r, targetURL)
		return
	}

	entry, reader, err := p.storage.Get(targetURL)
	if err == nil {
		log.Printf("[CACHE HIT] %s (age: %v)", targetURL, time.Since(entry.CreatedAt).Round(time.Second))
		p.serveCached(w, entry, reader)
		return
	}

	log.Printf("[CACHE MISS] %s", targetURL)
	p.fetchAndCache(w, r, targetURL)
}

func (p *Proxy) serveCached(w http.ResponseWriter, entry *cache.CacheEntry, reader io.ReadCloser) {
	defer reader.Close()

	w.Header().Set("Content-Type", entry.ContentType)
	for k, v := range entry.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("X-Cache", "HIT")
	w.Header().Set("X-Cache-Created", entry.CreatedAt.Format(time.RFC3339))

	io.Copy(w, reader)
}

func (p *Proxy) fetchAndCache(w http.ResponseWriter, r *http.Request, targetURL string) {
	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for k, v := range r.Header {
		req.Header[k] = v
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch %s: %v", targetURL, err)
		http.Error(w, "Failed to fetch resource", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	shouldCache := resp.StatusCode == http.StatusOK &&
		(r.Method == http.MethodGet || r.Method == http.MethodHead)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(resp.StatusCode)

	if !shouldCache {
		io.Copy(w, resp.Body)
		return
	}

	ttl := p.getTTL(targetURL, resp.Header)
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	expectedSize := resp.ContentLength

	pr, pw := io.Pipe()
	tee := io.TeeReader(resp.Body, pw)

	errChan := make(chan error, 1)
	go func() {
		defer pw.Close()
		_, err := io.Copy(w, tee)
		if err != nil {
			errChan <- err
		}
		close(errChan)
	}()

	contentType := resp.Header.Get("Content-Type")
	err = p.storage.Put(targetURL, contentType, headers, ttl, pr, expectedSize)
	if err != nil {
		if strings.Contains(err.Error(), "incomplete download") || strings.Contains(err.Error(), "empty file") {
			log.Printf("[CACHE ERROR] %s: %v", targetURL, err)
		} else if strings.Contains(err.Error(), "too small") {
			log.Printf("[CACHE SKIP] %s: file too small", targetURL)
		} else if strings.Contains(err.Error(), "too large") {
			log.Printf("[CACHE SKIP] %s: file too large", targetURL)
		} else {
			log.Printf("[CACHE WARNING] %s: %v", targetURL, err)
		}
	} else {
		log.Printf("[CACHE STORED] %s (ttl: %v, size: %d bytes)", targetURL, ttl.Round(time.Second), expectedSize)
	}

	if err := <-errChan; err != nil {
		log.Printf("[ERROR] Failed to write response: %v", err)
	}
}

func (p *Proxy) forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for k, v := range r.Header {
		req.Header[k] = v
	}

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to forward %s: %v", targetURL, err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if !p.rules.ShouldAllowHTTPS(host) {
		log.Printf("[CONNECT BLOCKED] %s (not in https_passthrough)", r.Host)
		http.Error(w, "CONNECT not allowed for this destination", http.StatusForbidden)
		return
	}

	log.Printf("[CONNECT ALLOWED] %s", r.Host)

	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to destination", http.StatusBadGateway)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(destConn, clientConn)
	io.Copy(clientConn, destConn)
}

func (p *Proxy) getTTL(url string, headers http.Header) time.Duration {
	ttl := p.rules.GetTTL(url, p.config.Cache.DefaultTTL)

	if !p.config.Cache.RespectHeaders {
		return ttl
	}

	cacheControl := headers.Get("Cache-Control")
	if cacheControl != "" {
		if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") {
			return 0
		}

		if strings.Contains(cacheControl, "max-age=") {
			var maxAge int
			fmt.Sscanf(cacheControl, "max-age=%d", &maxAge)
			if maxAge > 0 {
				headerTTL := time.Duration(maxAge) * time.Second
				if headerTTL < ttl {
					return headerTTL
				}
			}
		}
	}

	return ttl
}
