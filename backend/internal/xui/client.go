package xui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
)

type Client struct {
	BaseURL  string
	APIKey   string
	Username string
	Password string
	HTTP     *http.Client
}

func NewClient(baseURL, apiKey, username, password string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		APIKey:   apiKey,
		Username: username,
		Password: password,
		HTTP:     &http.Client{Jar: jar},
	}
}

func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	return c.HTTP.Do(req)
}

func (c *Client) ListInbounds(ctx context.Context) ([]InboundRecord, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("xui base url is not configured")
	}

	var lastErr error
	for _, endpoint := range c.inboundListCandidates() {
		rows, _, err := c.listInboundsAt(ctx, endpoint)
		if err == nil {
			return rows, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("xui list inbounds returned no candidates")
}

func (c *Client) joinURL(nextPath string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse xui base url: %w", err)
	}
	base.Path = path.Join(base.Path, nextPath)
	return base.String(), nil
}

func (c *Client) inboundListCandidates() []string {
	candidates := make([]string, 0, 3)
	seen := map[string]struct{}{}

	add := func(p string) {
		endpoint, err := c.joinURL(p)
		if err != nil {
			return
		}
		if _, ok := seen[endpoint]; ok {
			return
		}
		seen[endpoint] = struct{}{}
		candidates = append(candidates, endpoint)
	}

	add("/panel/api/inbounds/list")
	add("/xui/panel/api/inbounds/list")
	add("/api/inbounds/list")

	return candidates
}

func (c *Client) DetectInboundEndpoint(ctx context.Context) (string, []InboundRecord, error) {
	var lastErr error
	for _, endpoint := range c.inboundListCandidates() {
		rows, detected, err := c.listInboundsAt(ctx, endpoint)
		if err == nil {
			return detected, rows, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", nil, lastErr
	}
	return "", nil, fmt.Errorf("xui list inbounds returned no candidates")
}

func (c *Client) listInboundsAt(ctx context.Context, endpoint string) ([]InboundRecord, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("xui list inbounds at %s returned %s", endpoint, resp.Status)
	}

	var payload inboundListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("decode inbounds response: %w", err)
	}

	if len(payload.RawObj) > 0 {
		var rows []InboundRecord
		if err := json.Unmarshal(payload.RawObj, &rows); err == nil && len(rows) > 0 {
			return rows, endpoint, nil
		}
	}
	if len(payload.RawData) > 0 {
		var rows []InboundRecord
		if err := json.Unmarshal(payload.RawData, &rows); err == nil && len(rows) > 0 {
			return rows, endpoint, nil
		}
	}

	return nil, "", fmt.Errorf("xui list inbounds at %s returned empty payload", endpoint)
}
