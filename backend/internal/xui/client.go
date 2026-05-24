package xui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"
)

type apiResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

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
		HTTP: &http.Client{
			Jar:     jar,
			Timeout: 15 * time.Second,
		},
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
	return c.candidateURLs(
		"/panel/api/inbounds/list",
		"/xui/panel/api/inbounds/list",
		"/api/inbounds/list",
	)
}

func (c *Client) inboundActionCandidates(action string) []string {
	return c.candidateURLs(
		"/panel/api/inbounds/"+strings.TrimLeft(action, "/"),
		"/xui/panel/api/inbounds/"+strings.TrimLeft(action, "/"),
		"/api/inbounds/"+strings.TrimLeft(action, "/"),
	)
}

func (c *Client) serverActionCandidates(action string) []string {
	return c.candidateURLs(
		"/panel/api/server/"+strings.TrimLeft(action, "/"),
		"/xui/panel/api/server/"+strings.TrimLeft(action, "/"),
		"/api/server/"+strings.TrimLeft(action, "/"),
	)
}

func (c *Client) candidateURLs(paths ...string) []string {
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

	for _, p := range paths {
		add(p)
	}

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

// AddClient adds one or more clients to an inbound.
// settings is the JSON-encoded settings.clients array.
func (c *Client) AddClient(ctx context.Context, inboundID int, settings string) error {
	// The 3x-ui API expects settings as a JSON-encoded string value,
	// not a raw JSON object. json.Marshal does the proper escaping.
	settingsEncoded, _ := json.Marshal(settings)
	body := fmt.Sprintf(`{"id":%d,"settings":%s}`, inboundID, string(settingsEncoded))
	return c.postJSONCandidates(ctx, c.inboundActionCandidates("addClient"), body, "add client")
}

// UpdateClient updates a single client's configuration.
// clientID is the UUID/password of the client to update.
// settings is the JSON-encoded inbound settings with updated client.
func (c *Client) UpdateClient(ctx context.Context, clientID string, inboundID int, settings string) error {
	settingsEncoded, _ := json.Marshal(settings)
	body := fmt.Sprintf(`{"id":%d,"settings":%s}`, inboundID, string(settingsEncoded))
	return c.postJSONCandidates(ctx, c.inboundActionCandidates("updateClient/"+url.PathEscape(clientID)), body, "update client")
}

// DeleteClient removes a client from an inbound by its UUID/password.
func (c *Client) DeleteClient(ctx context.Context, inboundID int, clientID string) error {
	action := fmt.Sprintf("%d/delClient/%s", inboundID, url.PathEscape(clientID))
	return c.postJSONCandidates(ctx, c.inboundActionCandidates(action), "", "delete client")
}

// DeleteClientByEmail removes a client from an inbound by email.
func (c *Client) DeleteClientByEmail(ctx context.Context, inboundID int, email string) error {
	action := fmt.Sprintf("%d/delClientByEmail/%s", inboundID, url.PathEscape(email))
	return c.postJSONCandidates(ctx, c.inboundActionCandidates(action), "", "delete client by email")
}

// RestartXray triggers a Xray restart on the panel.
func (c *Client) RestartXray(ctx context.Context) error {
	var lastErr error
	for _, endpoint := range c.serverActionCandidates("restartXrayService") {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
		if err != nil {
			return err
		}

		resp, err := c.Do(ctx, req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// 202 Accepted is also valid here
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
			return nil
		}
		lastErr = fmt.Errorf("restart xray at %s returned %s", endpoint, resp.Status)
	}
	return lastErr
}

func (c *Client) postJSONCandidates(ctx context.Context, endpoints []string, body string, action string) error {
	var lastErr error
	for _, endpoint := range endpoints {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
		if err != nil {
			return err
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.Do(ctx, req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s at %s returned %s", action, endpoint, resp.Status)
			continue
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			return nil
		}

		var apiResp apiResponse
		if err := json.Unmarshal(raw, &apiResp); err != nil {
			return nil
		}
		if !apiResp.Success {
			return fmt.Errorf("%s rejected: %s", action, apiResp.Msg)
		}
		return nil
	}
	return lastErr
}
