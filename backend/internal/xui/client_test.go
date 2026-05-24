package xui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddClientTriesXUIPrefixedPath(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xui/panel/api/inbounds/addClient" {
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

	if err := client.AddClient(context.Background(), 7, `{"clients":[{"email":"a@example.com"}]}`); err != nil {
		t.Fatalf("AddClient returned error: %v", err)
	}
	if gotPath != "/xui/panel/api/inbounds/addClient" {
		t.Fatalf("expected xui-prefixed path, got %q", gotPath)
	}
	if gotBody["id"] != float64(7) {
		t.Fatalf("expected inbound id in body, got %#v", gotBody)
	}
	settings, ok := gotBody["settings"].(string)
	if !ok || !strings.Contains(settings, "a@example.com") {
		t.Fatalf("expected encoded settings string, got %#v", gotBody["settings"])
	}
}

func TestDeleteClientByEmailEscapesEmailAndTriesFallbacks(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/inbounds/9/delClientByEmail/user@example.com" {
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

	if err := client.DeleteClientByEmail(context.Background(), 9, "user@example.com"); err != nil {
		t.Fatalf("DeleteClientByEmail returned error: %v", err)
	}
	if gotPath != "/api/inbounds/9/delClientByEmail/user@example.com" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}
