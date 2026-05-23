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

// AddClient adds one or more clients to an inbound.
// settings is the JSON-encoded settings.clients array.
func (c *Client) AddClient(ctx context.Context, inboundID int, settings string) error {
	endpoint, err := c.joinURL("/panel/api/inbounds/addClient")
	if err != nil {
		return err
	}

	// The 3x-ui API expects settings as a JSON-encoded string value,
	// not a raw JSON object. json.Marshal does the proper escaping.
	settingsEncoded, _ := json.Marshal(settings)
	body := fmt.Sprintf(`{"id":%d,"settings":%s}`, inboundID, string(settingsEncoded))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("add client: %s", resp.Status)
	}
	// 3x-ui returns HTTP 200 even on failure, check body
	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil {
		if !apiResp.Success {
			return fmt.Errorf("add client rejected: %s", apiResp.Msg)
		}
	}
	return nil
}

// UpdateClient updates a single client's configuration.
// clientID is the UUID/password of the client to update.
// settings is the JSON-encoded inbound settings with updated client.
func (c *Client) UpdateClient(ctx context.Context, clientID string, inboundID int, settings string) error {
	endpoint, err := c.joinURL("/panel/api/inbounds/updateClient/" + clientID)
	if err != nil {
		return err
	}

	settingsEncoded, _ := json.Marshal(settings)
	body := fmt.Sprintf(`{"id":%d,"settings":%s}`, inboundID, string(settingsEncoded))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update client: %s", resp.Status)
	}
	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil {
		if !apiResp.Success {
			return fmt.Errorf("update client rejected: %s", apiResp.Msg)
		}
	}
	return nil
}

// DeleteClient removes a client from an inbound by its UUID/password.
func (c *Client) DeleteClient(ctx context.Context, inboundID int, clientID string) error {
	endpoint, err := c.joinURL(fmt.Sprintf("/panel/api/inbounds/%d/delClient/%s", inboundID, clientID))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete client: %s", resp.Status)
	}
	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil {
		if !apiResp.Success {
			return fmt.Errorf("delete client rejected: %s", apiResp.Msg)
		}
	}
	return nil
}

// DeleteClientByEmail removes a client from an inbound by email.
func (c *Client) DeleteClientByEmail(ctx context.Context, inboundID int, email string) error {
	endpoint, err := c.joinURL(fmt.Sprintf("/panel/api/inbounds/%d/delClientByEmail/%s", inboundID, url.PathEscape(email)))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete client by email: %s", resp.Status)
	}
	return nil
}

// RestartXray triggers a Xray restart on the panel.
func (c *Client) RestartXray(ctx context.Context) error {
	endpoint, err := c.joinURL("/panel/api/server/restartXrayService")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 202 Accepted is also valid here
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("restart xray: %s", resp.Status)
	}
	return nil
}
