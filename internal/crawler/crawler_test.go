
package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchHTML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("<html><title>x</title></html>"))
	}))
	defer ts.Close()

	client := NewHTTPClient(5*time.Second, 2*time.Second, 1024)
	rc, final, ct, dur, err := client.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetch err: %v", err)
	}
	defer rc.Close()
	if final == "" || ct == "" || dur == 0 {
		t.Fatal("unexpected empty values")
	}
}

func TestRejectNonHTML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	client := NewHTTPClient(5*time.Second, 2*time.Second, 1024)
	_, _, _, _, err := client.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error for non-html")
	}
}
