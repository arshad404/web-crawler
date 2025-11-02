
package classifier

import (
	"testing"

	"brightedge-go-crawler/internal/models"
)

func TestClassify(t *testing.T) {
	cl := New()
	page := models.Page{Content: models.Content{Text: "This is a product page. Buy now for $10. Add to cart to continue."}}
	c := cl.Classify(page)
	if c.Label != "product" {
		t.Fatalf("want product, got %s", c.Label)
	}
	news := models.Page{Content: models.Content{Text: "Published today by author John. 5 minutes read."}}
	c2 := cl.Classify(news)
	if c2.Label != "news" {
		t.Fatalf("want news, got %s", c2.Label)
	}
	topics := cl.TopTopics("go go network network network parsing parsing", 3)
	if len(topics) == 0 || topics[0] != "network" {
		t.Fatalf("unexpected topics: %#v", topics)
	}
}
