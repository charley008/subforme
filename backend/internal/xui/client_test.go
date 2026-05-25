package xui

import (
	"context"
	"encoding/json"
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
}

func TestUpdateClientByEmailEscapesEmailAndTriesFallbacks(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clients/update/user@example.com" {
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
	if gotPath != "/api/clients/update/user@example.com" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}

func TestAddInboundUsesFormAndStripsClients(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/inbounds/add" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
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
	if gotForm.Get("protocol") != "vless" {
		t.Fatalf("unexpected form: %#v", gotForm)
	}
	if strings.Contains(gotForm.Get("settings"), `"email":"a"`) {
		t.Fatalf("expected clients to be stripped, got %s", gotForm.Get("settings"))
	}
}
