package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

type EgressDialer struct {
	proxyType string
	proxyURL  string
	dialer    proxy.Dialer
}

func NewEgressDialer(proxyType, proxyURL string) (*EgressDialer, error) {
	if proxyType == "" || proxyURL == "" {
		return &EgressDialer{
			dialer: &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			},
		}, nil
	}

	var dialer proxy.Dialer

	switch proxyType {
	case "socks5":
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid SOCKS5 proxy URL: %w", err)
		}

		var auth *proxy.Auth
		if parsedURL.User != nil {
			password, _ := parsedURL.User.Password()
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: password,
			}
		}

		dialer, err = proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

	case "http":
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP proxy URL: %w", err)
		}

		dialer = &httpProxyDialer{
			proxyURL: parsedURL,
			direct: &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			},
		}

	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", proxyType)
	}

	return &EgressDialer{
		proxyType: proxyType,
		proxyURL:  proxyURL,
		dialer:    dialer,
	}, nil
}

func (e *EgressDialer) GetTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return e.dialer.Dial(network, addr)
		},
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ForceAttemptHTTP2:     false,
	}
}

type httpProxyDialer struct {
	proxyURL *url.URL
	direct   *net.Dialer
}

func (h *httpProxyDialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := h.direct.Dial("tcp", h.proxyURL.Host)
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	if h.proxyURL.User != nil {
		username := h.proxyURL.User.Username()
		password, _ := h.proxyURL.User.Password()
		req.SetBasicAuth(username, password)
	}

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write CONNECT request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("CONNECT failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	return conn, nil
}
