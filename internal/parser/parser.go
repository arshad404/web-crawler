
package parser

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"

	"brightedge-go-crawler/internal/models"
)

type Parser struct{}

func New() *Parser { return &Parser{} }

var whitespaceRe = regexp.MustCompile(`\s+`)

func (p *Parser) Extract(r io.Reader, contentType string) (models.Page, error) {
	// Decode to UTF-8 if needed
	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, r)
	data := buf.Bytes()

	enc, _, _ := charset.DetermineEncoding(data, contentType)
	utf8data, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		// fallback: if already utf-8, continue
		if !utf8.Valid(data) {
			return models.Page{}, err
		}
		utf8data = data
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(utf8data))
	if err != nil {
		return models.Page{}, err
	}

	// Remove script & style
	doc.Find("script,noscript,style").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	title := strings.TrimSpace(doc.Find("title").First().Text())
	desc := strings.TrimSpace(doc.Find(`meta[name="description"]`).AttrOr("content", ""))
	if desc == "" {
		desc = strings.TrimSpace(doc.Find(`meta[property="og:description"]`).AttrOr("content", ""))
	}

	// keywords
	var keywords []string
	if kw := doc.Find(`meta[name="keywords"]`).AttrOr("content", ""); kw != "" {
		for _, k := range strings.Split(kw, ",") {
			trim := strings.ToLower(strings.TrimSpace(k))
			if trim != "" {
				keywords = append(keywords, trim)
			}
		}
	}

	// OG tags
	og := map[string]string{}
	doc.Find(`meta[property^="og:"]`).Each(func(i int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")
		if prop != "" && content != "" {
			og[prop] = content
		}
	})

	canonical := strings.TrimSpace(doc.Find(`link[rel="canonical"]`).AttrOr("href", ""))

	// headings
	h1 := strings.TrimSpace(doc.Find("h1").First().Text())
	var h2s []string
	doc.Find("h2").Each(func(i int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		if txt != "" {
			h2s = append(h2s, txt)
		}
	})

	// main text: gather paragraphs and list items
	var parts []string
	doc.Find("p,li").Each(func(i int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if t != "" {
			parts = append(parts, t)
		}
	})
	text := strings.TrimSpace(whitespaceRe.ReplaceAllString(strings.Join(parts, " "), " "))
	wordCount := 0
	if text != "" {
		wordCount = len(strings.Fields(text))
	}

	// language detection (very light heuristic using <html lang> or og:locale)
	lang := strings.TrimSpace(doc.Find("html").AttrOr("lang", ""))
	if lang == "" {
		lang = og["og:locale"]
	}

	content := models.Content{
		Text:      text,
		WordCount: wordCount,
		Language:  lang,
	}
	// collect headings too
	doc.Find("h1,h2,h3").Each(func(i int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if t != "" {
			content.Headings = append(content.Headings, t)
		}
	})

	meta := models.Meta{
		Title:       title,
		Description: desc,
		Keywords:    keywords,
		OG:          og,
		Canonical:   canonical,
		H1:          h1,
		H2:          h2s,
	}

	return models.Page{Meta: meta, Content: content}, nil
}
