package RSS

import (
	"context"
	"encoding/xml"
	"html"
	"io"
	"log"
	"net/http"
)

func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	log.Printf("DEBUG: FetchFeed called with URL: %s", feedURL)
	feed := RSSFeed{}
	fdpnt := &feed

	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return fdpnt, err
	}
	req.Header.Set("User-Agent", "gator")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fdpnt, err
	}
	defer resp.Body.Close()

	dat, err := io.ReadAll(resp.Body)
	if err != nil {
		return fdpnt, err
	}

	err = xml.Unmarshal(dat, &feed)
	if err != nil {
		return fdpnt, err
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	return fdpnt, nil

}
