
package parser

import (
	"strings"
	"testing"
)

const sampleHTML = `<!doctype html><html lang="en"><head>
<title>Test Page</title>
<meta name="description" content="A short description">
<meta property="og:type" content="article">
</head><body>
<h1>Hello</h1><h2>Subtitle</h2>
<p>Go is great for network services.</p>
<p>Add to cart is not here.</p>
</body></html>`

func TestExtract(t *testing.T) {
	p := New()
	page, err := p.Extract(strings.NewReader(sampleHTML), "text/html; charset=utf-8")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if page.Meta.Title != "Test Page" {
		t.Fatalf("want title Test Page, got %q", page.Meta.Title)
	}
	if page.Content.WordCount == 0 {
		t.Fatal("expected non-zero word count")
	}
	if page.Meta.OG["og:type"] != "article" {
		t.Fatal("og:type missing")
	}
}
