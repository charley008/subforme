package xui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCreateClientUsesNewClientAPI(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/clients/add" {
			http.NotFound(w, r)
			return
		}
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()

	err := client.CreateClient(context.Background(), InboundClient{Email: "a@example.com", ID: "uuid-1", Enable: true}, []int{7})
	if err != nil {
		t.Fatalf("CreateClient returned error: %v", err)
	}
	if gotPath != "/panel/api/clients/add" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if gotBody["client"].(map[string]any)["email"] != "a@example.com" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
	if gotBody["inboundIds"].([]any)[0] != float64(7) {
		t.Fatalf("expected inbound id, got %#v", gotBody)
	}
}

func TestNewClientDoesNotUseEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
	client := NewClient("https://panel.example.com/xui", "token-1", "", "")

	transport, ok := client.HTTP.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.HTTP.Transport)
	}
	if transport.Proxy == nil {
		return
	}
	reqURL, _ := url.Parse("https://panel.example.com/xui/panel/api/inbounds/list")
	proxyURL, err := transport.Proxy(&http.Request{URL: reqURL})
	if err != nil {
		t.Fatalf("proxy lookup returned error: %v", err)
	}
	if proxyURL != nil {
		t.Fatalf("expected no proxy for xui client, got %v", proxyURL)
	}
}

func TestListClientsReadsGlobalClientAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/clients/list" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":[{"email":"arzy","uuid":"uuid-1","subId":"sub-1","inboundIds":[1]}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()

	rows, err := client.ListClients(context.Background())
	if err != nil {
		t.Fatalf("ListClients returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != "arzy" || rows[0].InboundIDs[0] != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if rows[0].Traffic.Up != 0 || rows[0].Traffic.Down != 0 {
		t.Fatalf("expected zero traffic defaults, got %#v", rows[0].Traffic)
	}
}

func TestCandidateURLsWithBasePathAvoidsNestedXUIPath(t *testing.T) {
	client := NewClient("https://panel.example.com/xui", "token-1", "", "")

	got := client.inboundListCandidates()
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "/xui/xui/") {
		t.Fatalf("candidate URLs should not contain nested xui path: %#v", got)
	}

	wantOrder := []string{
		"https://panel.example.com/xui/panel/api/inbounds/list",
		"https://panel.example.com/panel/api/inbounds/list",
	}
	if len(got) != len(wantOrder) {
		t.Fatalf("unexpected candidate count: %#v", got)
	}
	for i, want := range wantOrder {
		if got[i] != want {
			t.Fatalf("candidate %d: expected %q, got %q; all=%#v", i, want, got[i], got)
		}
	}
}

func TestResetAllClientTrafficsUsesGlobalEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/clients/resetAllTraffics" {
			http.NotFound(w, r)
			return
		}
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()

	if err := client.ResetAllClientTraffics(context.Background()); err != nil {
		t.Fatalf("ResetAllClientTraffics returned error: %v", err)
	}
	if gotPath != "/panel/api/clients/resetAllTraffics" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}

func TestUpdateClientByEmailEscapesEmailAndTriesFallbacks(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/clients/update/user@example.com" {
			http.NotFound(w, r)
			return
		}
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()

	err := client.UpdateClientByEmail(context.Background(), "user@example.com", InboundClient{Email: "user@example.com", ID: "uuid-1", Enable: true})
	if err != nil {
		t.Fatalf("UpdateClientByEmail returned error: %v", err)
	}
	if gotPath != "/panel/api/clients/update/user@example.com" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}

func TestUpdateInboundWithClientsKeepsSettingsClients(t *testing.T) {
	var payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/inbounds/update/1" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		payload = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()
	err := client.UpdateInboundWithClients(context.Background(), 1, InboundRecord{
		Remark:   "vless",
		Enable:   true,
		Listen:   "127.0.0.1",
		Port:     6443,
		Protocol: "vless",
		Settings: `{"clients":[{"email":"charley","id":"uuid-1","flow":"xtls-rprx-vision","enable":true}],"decryption":"none"}`,
	})
	if err != nil {
		t.Fatalf("UpdateInboundWithClients returned error: %v", err)
	}
	if !strings.Contains(payload, `"flow":"xtls-rprx-vision"`) {
		t.Fatalf("expected payload to preserve clients, got %s", payload)
	}
}

func TestAddInboundUsesJSONAndStripsClients(t *testing.T) {
	var gotBody map[string]any
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/inbounds/add" {
			http.NotFound(w, r)
			return
		}
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-1", "", "")
	client.HTTP = server.Client()

	err := client.AddInbound(context.Background(), InboundRecord{
		Remark:         "vless",
		Enable:         true,
		Port:           443,
		Protocol:       "vless",
		Settings:       `{"clients":[{"email":"a"}],"decryption":"none"}`,
		StreamSettings: `{}`,
		Tag:            "inbound-127.0.0.1:443",
	})
	if err != nil {
		t.Fatalf("AddInbound returned error: %v", err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected json content type, got %q", gotContentType)
	}
	if gotBody["protocol"] != "vless" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
	settings := gotBody["settings"].(map[string]any)
	if len(settings["clients"].([]any)) != 0 {
		t.Fatalf("expected clients to be stripped, got %#v", settings["clients"])
	}
}
