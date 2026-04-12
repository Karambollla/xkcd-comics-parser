package xkcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Karambollla/course/update/core"
)

const lastPath = "/info.0.json"

type Client struct {
	log    *slog.Logger
	client http.Client
	url    string
}

func NewClient(url string, timeout time.Duration, log *slog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("empty base url specified")
	}
	return &Client{
		client: http.Client{Timeout: timeout},
		log:    log,
		url:    url,
	}, nil
}

func (c Client) Get(ctx context.Context, id int) (core.XKCDInfo, error) {
	url := fmt.Sprintf("%s/%d%s", strings.TrimRight(c.url, "/"), id, lastPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return core.XKCDInfo{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("couldnt do a comics json", "id", id, "error", err)
		return core.XKCDInfo{}, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return core.XKCDInfo{}, core.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return core.XKCDInfo{}, fmt.Errorf("unexpected xkcd status %d for id %d", resp.StatusCode, id)
	}

	var raw struct {
		ID         int    `json:"num"`
		URL        string `json:"img"`
		Title      string `json:"title"`
		SafeTitle  string `json:"safe_title"`
		Transcript string `json:"transcript"`
		Alt        string `json:"alt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		c.log.Error("couldnt decode xkcd json", "error", err, "id", id)
		return core.XKCDInfo{}, err
	}

	description := fmt.Sprintf("%s %s %s %s", raw.Title, raw.SafeTitle, raw.Transcript, raw.Alt)
	return core.XKCDInfo{
		ID:          raw.ID,
		URL:         raw.URL,
		Description: description,
	}, nil
}

func (c Client) LastID(ctx context.Context) (int, error) {
	url := strings.TrimRight(c.url, "/") + lastPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.log.Error("couldnt fetch xkcd total number", "error", err)
		return 0, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("couldnt do a total comics json", "error", err)
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected xkcd status %d for last id", resp.StatusCode)
	}

	var raw struct {
		ID int `json:"num"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		c.log.Error("couldnt decode total number json", "error", err)
		return 0, err
	}

	return raw.ID, nil
}

func (c *Client) Close() error {
	c.client.CloseIdleConnections()
	return nil
}
