package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"brightedge-go-crawler/internal/classifier"
	"brightedge-go-crawler/internal/crawler"
	"brightedge-go-crawler/internal/ioformats"
	"brightedge-go-crawler/internal/models"
	"brightedge-go-crawler/internal/parser"
	"brightedge-go-crawler/pkg/logger"
)

type crawlReq struct {
	URL string `json:"url"`
}

type batchReq struct {
	URLs []string `json:"urls"`
}

func main() {
	l := logger.New()
	mux := http.NewServeMux()

	client := crawler.NewHTTPClient(15*time.Second, 5*time.Second, 5*1024*1024) // 5MB cap
	par := parser.New()
	cl := classifier.New()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// POST /crawl  { "url": "https://..." }
	mux.HandleFunc("/crawl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req crawlReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		body, finalURL, ct, fetchMs, err := client.Fetch(ctx, req.URL)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		defer body.Close()

		page, err := par.Extract(body, ct)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}

		result := models.CrawlResult{
			SourceURL: finalURL,
			FetchMs:   fetchMs.Milliseconds(),
			Meta:      page.Meta,
			Content:   page.Content,
		}
		result.Class = cl.Classify(page)
		result.Topics = cl.TopTopics(page.Content.Text, 15)

		writeJSON(w, http.StatusOK, result)
	})

	// POST /crawl/batch  { "urls": ["https://...", "..."] }
	mux.HandleFunc("/crawl/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req batchReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.URLs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		type out struct {
			URL    string              `json:"url"`
			Result *models.CrawlResult `json:"result,omitempty"`
			Error  string              `json:"error,omitempty"`
		}

		results := make([]out, len(req.URLs))

		// bounded concurrency
		sem := make(chan struct{}, 10)
		done := make(chan int, len(req.URLs))

		for i, u := range req.URLs {
			i, u := i, u
			sem <- struct{}{} // acquire
			go func() {
				defer func() { <-sem; done <- i }()
				ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
				defer cancel()
				if u == "" {
					results[i] = out{URL: u, Error: "empty url"}
					return
				}
				body, finalURL, ct, fetchMs, err := client.Fetch(ctx, u)
				if err != nil {
					results[i] = out{URL: u, Error: err.Error()}
					return
				}
				defer body.Close()
				page, err := par.Extract(body, ct)
				if err != nil {
					results[i] = out{URL: u, Error: err.Error()}
					return
				}
				cr := models.CrawlResult{
					SourceURL: finalURL,
					FetchMs:   fetchMs.Milliseconds(),
					Meta:      page.Meta,
					Content:   page.Content,
				}
				cr.Class = cl.Classify(page)
				cr.Topics = cl.TopTopics(page.Content.Text, 15)
				results[i] = out{URL: u, Result: &cr}
			}()
		}
		// wait
		for range req.URLs {
			<-done
		}
		writeJSON(w, http.StatusOK, results)
	})

	// POST /crawl/upload (multipart file=...) -> NDJSON stream
	mux.HandleFunc("/crawl/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart parse error"})
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file part 'file' required"})
			return
		}
		defer f.Close()

		// copy to temp file to reuse format reader
		tmp, err := os.CreateTemp("", "upload-*")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "temp file error"})
			return
		}
		if _, err := io.Copy(tmp, f); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "copy error"})
			return
		}
		tmp.Close()
		defer os.Remove(tmp.Name())

		urls, err := ioformats.ReadURLs(tmp.Name())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)

		type out struct {
			URL    string              `json:"url"`
			Result *models.CrawlResult `json:"result,omitempty"`
			Error  string              `json:"error,omitempty"`
		}

		sem := make(chan struct{}, 10)
		done := make(chan struct{})

		go func() {
			for _, u := range urls {
				sem <- struct{}{} // acquire
				u := u
				go func() {
					defer func() { <-sem }()
					ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
					defer cancel()
					body, finalURL, ct, fetchMs, err := client.Fetch(ctx, u)
					if err != nil {
						_ = enc.Encode(out{URL: u, Error: err.Error()})
						return
					}
					defer body.Close()
					page, err := par.Extract(body, ct)
					if err != nil {
						_ = enc.Encode(out{URL: u, Error: err.Error()})
						return
					}
					cr := models.CrawlResult{
						SourceURL: finalURL,
						FetchMs:   fetchMs.Milliseconds(),
						Meta:      page.Meta,
						Content:   page.Content,
					}
					cr.Class = cl.Classify(page)
					cr.Topics = cl.TopTopics(page.Content.Text, 15)
					_ = enc.Encode(out{URL: u, Result: &cr})
				}()
			}
			// wait for outstanding
			for i := 0; i < cap(sem); i++ {
				sem <- struct{}{} // fill to block until all released
			}
			close(done)
		}()

		<-done
	})

	addr := ":8080"
	srv := &http.Server{
		Addr:         addr,
		Handler:      logRequest(l, mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		l.Infof("server listening on %s", addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			l.Errorf("server error: %v", err)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	l.Infof("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	l.Infof("bye")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func logRequest(l *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		l.Infof("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
