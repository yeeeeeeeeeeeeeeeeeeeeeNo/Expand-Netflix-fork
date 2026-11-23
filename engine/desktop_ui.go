package main

import (
	"crypto/tls"
	_ "embed"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed noir_ui.html
var noirEmbeddedUI []byte

//go:embed nocturne_icon.png
var nocturneEmbeddedIcon []byte

type cachedProxyImg struct {
	data        []byte
	contentType string
	failed      bool
	cachedAt    time.Time
}

var (
	pcImgCacheMu   sync.RWMutex
	pcImgCache     = make(map[string]cachedProxyImg)
	pcImgCacheSize int64
	pcImgClient    = &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 16,
			IdleConnTimeout:     45 * time.Second,
		},
	}
)

func (e *Engine) handleNoirUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(noirEmbeddedUI)
}

func (e *Engine) handleNocturneIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(nocturneEmbeddedIcon)
}

// handlePcImageProxy fetches TV channel logos server-side without browser Referer headers
// (fixes i.imgur.com 403 hotlink blocks) and with a compliant User-Agent (fixes Wikimedia 429),
// caching bytes in memory so scrolling hundreds of channels is instantaneous.
func (e *Engine) handlePcImageProxy(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("u"))
	if rawURL == "" || (!strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://")) {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	pcImgCacheMu.RLock()
	entry, hit := pcImgCache[rawURL]
	pcImgCacheMu.RUnlock()

	if hit {
		if entry.failed {
			if time.Since(entry.cachedAt) < 10*time.Minute {
				http.NotFound(w, r)
				return
			}
		} else {
			w.Header().Set("Content-Type", entry.contentType)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(entry.data)
			return
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	low := strings.ToLower(rawURL)
	if strings.Contains(low, "wikimedia.org") || strings.Contains(low, "wikipedia.org") {
		req.Header.Set("User-Agent", "NocturneTV/3.2 (https://github.com/yeeeeeeeeeeeeeeeeeeeeeNo/Expand-Netflix-fork; contact@nocturne.local)")
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,image/*,*/*;q=0.8")
	// Deliberately omit Referer so i.imgur.com & IPTV CDNs never return 403 Forbidden.

	resp, err := pcImgClient.Do(req)
	if err != nil {
		cacheProxyImgFailure(rawURL)
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		cacheProxyImgFailure(rawURL)
		http.NotFound(w, r)
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil || len(data) < 32 {
		cacheProxyImgFailure(rawURL)
		http.NotFound(w, r)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" || strings.HasPrefix(ct, "application/octet-stream") {
		ct = http.DetectContentType(data)
		if strings.HasSuffix(low, ".svg") || strings.Contains(string(data[:minInt(256, len(data))]), "<svg") {
			ct = "image/svg+xml"
		}
	}
	if strings.Contains(strings.ToLower(ct), "text/html") {
		cacheProxyImgFailure(rawURL)
		http.NotFound(w, r)
		return
	}

	pcImgCacheMu.Lock()
	if pcImgCacheSize > 32*1024*1024 || len(pcImgCache) > 600 {
		pcImgCache = make(map[string]cachedProxyImg)
		pcImgCacheSize = 0
	}
	pcImgCache[rawURL] = cachedProxyImg{
		data:        data,
		contentType: ct,
		failed:      false,
		cachedAt:    time.Now(),
	}
	pcImgCacheSize += int64(len(data))
	pcImgCacheMu.Unlock()

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func cacheProxyImgFailure(rawURL string) {
	pcImgCacheMu.Lock()
	if len(pcImgCache) > 800 {
		pcImgCache = make(map[string]cachedProxyImg)
		pcImgCacheSize = 0
	}
	pcImgCache[rawURL] = cachedProxyImg{failed: true, cachedAt: time.Now()}
	pcImgCacheMu.Unlock()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
