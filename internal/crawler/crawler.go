
package crawler

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPClient struct {
	client     *http.Client
	sizeCap    int64
	userAgent  string
}

func NewHTTPClient(timeout, dialTimeout time.Duration, sizeCap int64) *HTTPClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		sizeCap:   sizeCap,
		userAgent: "brightedge-go-crawler/1.0 (+https://example.com)",
	}
}

func (h *HTTPClient) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, string, string, time.Duration, error) {
	start := time.Now()
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, "", "", 0, fmt.Errorf("invalid url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", "", 0, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", h.userAgent)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, "", "", 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, "", "", 0, fmt.Errorf("http status %d", resp.StatusCode)
	}

	var body io.ReadCloser = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, "", "", 0, err
		}
		body = gz
	}

	// enforce a size cap
	r := io.LimitReader(body, h.sizeCap)
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if !strings.Contains(mediaType, "text/html") && !strings.Contains(mediaType, "application/xhtml+xml") && mediaType != "" {
		// still allow if empty (some servers omit), otherwise reject non-html
		body.Close()
		return nil, "", "", 0, errors.New("non-html content")
	}

	finalURL := resp.Request.URL.String()
	elapsed := time.Since(start)
	return io.NopCloser(r), finalURL, contentType, elapsed, nil
}
