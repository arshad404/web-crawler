
package ioformats

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadURLs reads URLs from a CSV (expects header with "url") or NDJSON file.
// If ext cannot be determined, tries CSV first then NDJSON.
func ReadURLs(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return readCSV(path)
	case ".ndjson", ".jsonl":
		return readNDJSON(path)
	default:
		// try csv then ndjson
		if urls, err := readCSV(path); err == nil && len(urls) > 0 {
			return urls, nil
		}
		return readNDJSON(path)
	}
}

func readCSV(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	// find "url" column
	col := -1
	for i, h := range rows[0] {
		if strings.EqualFold(strings.TrimSpace(h), "url") {
			col = i
			break
		}
	}
	if col == -1 {
		return nil, errors.New("csv must contain a 'url' header column")
	}
	var out []string
	for _, row := range rows[1:] {
		if col < len(row) {
			u := strings.TrimSpace(row[col])
			if u != "" {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

func readNDJSON(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// allow raw string or {"url": "..."}
		if strings.HasPrefix(line, "{") {
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err == nil {
				if v, ok := obj["url"]; ok {
					if s, ok := v.(string); ok && s != "" {
						out = append(out, s)
						continue
					}
				}
			}
		}
		// fallback: treat whole line as url
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no urls found in ndjson")
	}
	return out, nil
}

// WriteNDJSON writes any JSON-marshalable items as NDJSON to w.
func WriteNDJSON(w io.Writer, items []any) error {
	enc := json.NewEncoder(w)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}
