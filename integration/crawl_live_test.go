
//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"brightedge-go-crawler/internal/classifier"
	"brightedge-go-crawler/internal/crawler"
	"brightedge-go-crawler/internal/parser"
)

func TestAmazonProductPage(t *testing.T) {
	// Amazon toaster product page (subject to change / blocking)
	url := "https://www.amazon.com/Cuisinart-CPT-122-Compact-2-Slice-Toaster/dp/B009GQ034C"

	client := crawler.NewHTTPClient(25*time.Second, 5*time.Second, 5*1024*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	body, _, ct, _, err := client.Fetch(ctx, url)
	if err != nil {
		t.Skipf("skipping: fetch failed due to network/robots/captcha: %v", err)
		return
	}
	defer body.Close()

	p := parser.New()
	page, err := p.Extract(body, ct)
	if err != nil {
		t.Skipf("skipping: parse failed: %v", err)
		return
	}

	cl := classifier.New()
	class := cl.Classify(page)
	if class.Label != "product" {
		t.Errorf("expected product class, got %s", class.Label)
	}
	if len(cl.TopTopics(page.Content.Text, 10)) == 0 {
		t.Errorf("expected non-empty topics")
	}
}
