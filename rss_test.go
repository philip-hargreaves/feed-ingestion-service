package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchFeedParsesAndUnescapes(t *testing.T) {
	var capturedUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`
<rss version="2.0">
  <channel>
    <title>Feed &amp; Title</title>
    <link>https://example.com</link>
    <description>Tom &amp; Jerry</description>
    <item>
      <title>Item &amp; Title</title>
      <link>https://example.com/post-1</link>
      <description>Some &amp; item description</description>
      <pubDate>Mon, 06 Sep 2021 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	feed, err := fetchFeed(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedUserAgent != "feeder" {
		t.Fatalf("expected User-Agent feeder, got %q", capturedUserAgent)
	}
	if feed.Channel.Title != "Feed & Title" {
		t.Fatalf("expected unescaped channel title, got %q", feed.Channel.Title)
	}
	if feed.Channel.Description != "Tom & Jerry" {
		t.Fatalf("expected unescaped channel description, got %q", feed.Channel.Description)
	}
	if len(feed.Channel.Item) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Channel.Item))
	}
	if feed.Channel.Item[0].Title != "Item & Title" {
		t.Fatalf("expected unescaped item title, got %q", feed.Channel.Item[0].Title)
	}
	if feed.Channel.Item[0].Description != "Some & item description" {
		t.Fatalf("expected unescaped item description, got %q", feed.Channel.Item[0].Description)
	}
}

func TestFetchFeedInvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<rss><channel><title>broken`))
	}))
	defer server.Close()

	_, err := fetchFeed(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected XML parse error")
	}
	if !strings.Contains(err.Error(), "Couldn't parse feed XML") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchFeedAcceptsRSaquoEntity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`
<rss version="2.0">
  <channel>
    <title>Planet &rsaquo; F1</title>
    <link>https://www.planetf1.com</link>
    <description>News &amp; analysis</description>
    <item>
      <title>Latest &rsaquo; Headline</title>
      <link>https://www.planetf1.com/post</link>
      <description>Story body</description>
      <pubDate>Mon, 06 Sep 2021 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer server.Close()

	feed, err := fetchFeed(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if feed.Channel.Title != "Planet › F1" {
		t.Fatalf("expected rsaquo in title to decode, got %q", feed.Channel.Title)
	}
	if len(feed.Channel.Item) != 1 {
		t.Fatalf("expected one item, got %d", len(feed.Channel.Item))
	}
	if feed.Channel.Item[0].Title != "Latest › Headline" {
		t.Fatalf("expected rsaquo in item title to decode, got %q", feed.Channel.Item[0].Title)
	}
}
