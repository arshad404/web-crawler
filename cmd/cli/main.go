package main

import (
	"context"

	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"brightedge-go-crawler/internal/classifier"
	"brightedge-go-crawler/internal/crawler"
	"brightedge-go-crawler/internal/ioformats"
	"brightedge-go-crawler/internal/models"
	"brightedge-go-crawler/internal/parser"
)

func main() {
	in := flag.String("input", "", "input file (csv with 'url' column or ndjson)")
	out := flag.String("output", "", "output NDJSON file (default stdout)")
	concurrency := flag.Int("concurrency", 10, "worker concurrency")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "missing --input")
		os.Exit(2)
	}

	urls, err := ioformats.ReadURLs(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read input:", err)
		os.Exit(1)
	}

	client := crawler.NewHTTPClient(15*time.Second, 5*time.Second, 5*1024*1024)
	par := parser.New()
	cl := classifier.New()

	type outRec struct {
		URL    string              `json:"url"`
		Result *models.CrawlResult `json:"result,omitempty"`
		Error  string              `json:"error,omitempty"`
	}

	results := make([]outRec, len(urls))

	sem := make(chan struct{}, *concurrency)
	done := make(chan int, len(urls))

	for i, u := range urls {
		i, u := i, u
		sem <- struct{}{} // acquire
		go func() {
			defer func() { <-sem; done <- i }()
			body, finalURL, ct, fetchMs, err := client.Fetch(context.Background(), u)
			if err != nil {
				results[i] = outRec{URL: u, Error: err.Error()}
				return
			}
			defer body.Close()
			page, err := par.Extract(body, ct)
			if err != nil {
				results[i] = outRec{URL: u, Error: err.Error()}
				return
			}
			cr := models.CrawlResult{
				SourceURL: finalURL,
				FetchMs:   fetchMs.Milliseconds(),
				Meta:      page.Meta,
				Content:   page.Content,
				Class:     cl.Classify(page),
				Topics:    cl.TopTopics(page.Content.Text, 15),
			}
			results[i] = outRec{URL: u, Result: &cr}
		}()
	}
	for range urls {
		<-done
	}

	var w *os.File
	if *out == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create output:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	for _, r := range results {
		_ = enc.Encode(r)
	}
}
