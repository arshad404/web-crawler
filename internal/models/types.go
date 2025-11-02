
package models

type Meta struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	OG          map[string]string `json:"og,omitempty"`
	Canonical   string            `json:"canonical,omitempty"`
	H1          string            `json:"h1,omitempty"`
	H2          []string          `json:"h2,omitempty"`
}

type Content struct {
	Text      string   `json:"text,omitempty"`
	WordCount int      `json:"wordCount,omitempty"`
	Language  string   `json:"language,omitempty"`
	Headings  []string `json:"headings,omitempty"`
}

type Page struct {
	Meta    Meta    `json:"meta"`
	Content Content `json:"content"`
}

type Classification struct {
	Label  string            `json:"label"`
	Reason map[string]string `json:"reason,omitempty"`
}

type CrawlResult struct {
	SourceURL string         `json:"sourceUrl"`
	FetchMs   int64          `json:"fetchMs"`
	Meta      Meta           `json:"meta"`
	Content   Content        `json:"content"`
	Class     Classification `json:"class"`
	Topics    []string       `json:"topics"`
}
