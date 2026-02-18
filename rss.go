package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"
)

type RSSFeed struct {
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Item        []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create request: %w", err)
	}
	request.Header.Set("User-Agent", "feeder")

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Couldn't fetch feed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read response body: %w", err)
	}

	feed := RSSFeed{}
	decoder := xml.NewDecoder(bytes.NewReader(responseBody))
	decoder.Strict = false
	decoder.Entity = map[string]string{
		"lt":     "<",
		"gt":     ">",
		"amp":    "&",
		"apos":   "'",
		"quot":   "\"",
		"nbsp":   "\u00A0",
		"ldquo":  "\u201C",
		"rdquo":  "\u201D",
		"lsquo":  "\u2018",
		"rsquo":  "\u2019",
		"rsaquo": "\u203A",
		"ndash":  "\u2013",
		"mdash":  "\u2014",
	}
	err = decoder.Decode(&feed)
	if err != nil {
		return nil, fmt.Errorf("Couldn't parse feed XML: %w", err)
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
}
