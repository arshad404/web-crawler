
package classifier

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"brightedge-go-crawler/internal/models"
)

type Classifier struct{}

func New() *Classifier { return &Classifier{} }

// simple stopword list (extend as needed)
var stopwords = map[string]struct{}{
	"the": {}, "and": {}, "of": {}, "to": {}, "in": {}, "a": {}, "for": {}, "is": {}, "on": {}, "with": {}, "as": {},
	"by": {}, "at": {}, "from": {}, "that": {}, "this": {}, "it": {}, "an": {}, "be": {}, "or": {}, "are": {}, "was": {},
	"will": {}, "has": {}, "have": {}, "had": {}, "but": {}, "not": {}, "your": {}, "you": {}, "we": {}, "our": {},
}

var priceRe = regexp.MustCompile(`[$€£₹]\s?\d`)
var cartRe = regexp.MustCompile(`(?i)add\s+to\s+cart|buy\s+now|checkout`)
var articleRe = regexp.MustCompile(`(?i)author|byline|published|updated|minutes\s+read|subscribe`)

func (c *Classifier) Classify(p models.Page) models.Classification {
	text := strings.ToLower(p.Content.Text + " " + strings.Join(p.Content.Headings, " "))
	reason := map[string]string{}

	// product signals
	if priceRe.FindStringIndex(text) != nil {
		reason["price"] = "currency-like price detected"
	}
	if cartRe.FindStringIndex(text) != nil {
		reason["cart"] = "ecommerce CTA found"
	}
	if _, ok := p.Meta.OG["og:type"]; ok && strings.Contains(strings.ToLower(p.Meta.OG["og:type"]), "product") {
		reason["og:type"] = "og:type indicates product"
	}
	if len(reason) > 0 {
		return models.Classification{Label: "product", Reason: reason}
	}

	// news signals
	if strings.Contains(strings.ToLower(p.Meta.OG["og:type"]), "article") ||
		articleRe.FindStringIndex(text) != nil {
		reason["article"] = "article-like markers"
		return models.Classification{Label: "news", Reason: reason}
	}

	// blog signals
	if strings.Contains(text, "blog") || strings.Contains(strings.ToLower(p.Meta.Title), "blog") {
		reason["blog"] = "blog marker in title/content"
		return models.Classification{Label: "blog", Reason: reason}
	}

	return models.Classification{Label: "other", Reason: reason}
}

// TopTopics returns top N keywords by normalized frequency, ignoring stopwords and short tokens.
func (c *Classifier) TopTopics(text string, n int) []string {
	freq := map[string]int{}
	token := func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }
	words := strings.FieldsFunc(strings.ToLower(text), token)

	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if _, stop := stopwords[w]; stop {
			continue
		}
		freq[w]++
	}

	type kv struct {
		K string
		V int
	}
	var list []kv
	for k, v := range freq {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].V == list[j].V {
			return list[i].K < list[j].K
		}
		return list[i].V > list[j].V
	})
	if n > len(list) {
		n = len(list)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, list[i].K)
	}
	return out
}
