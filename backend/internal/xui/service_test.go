package xui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveUserPrefersEmailThenRemark(t *testing.T) {
	users := []UserRecord{
		{Email: "alpha@example.com", Remark: "alpha"},
		{Email: "beta@example.com", Remark: "charley"},
	}

	got, ok := ResolveUser(users, "charley")
	if !ok {
		t.Fatal("expected user match")
	}

	if got.Email != "beta@example.com" {
		t.Fatalf("expected beta@example.com, got %s", got.Email)
	}
}

func TestResolveUserReturnsFalseWhenMissing(t *testing.T) {
	_, ok := ResolveUser([]UserRecord{{Email: "a@example.com", Remark: "a"}}, "missing")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestResolverBuildsNodeFromInboundList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/api/inbounds/list" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":[{"id":1,"remark":"HK-01","enable":true,"listen":"","port":443,"protocol":"vless","settings":"{\"clients\":[{\"email\":\"charley\",\"id\":\"uuid-1\",\"flow\":\"xtls-rprx-vision\",\"enable\":true}]}","streamSettings":"{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{\"serverNames\":[\"www.cloudflare.com\"],\"shortIds\":[\"abcd1234\"],\"settings\":{\"publicKey\":\"pk-1\",\"fingerprint\":\"chrome\",\"serverName\":\"www.cloudflare.com\"}},\"tcpSettings\":{\"header\":{\"type\":\"none\"}}}"}]}`))
	}))
	defer server.Close()

	resolver := NewResolver(NewClient(server.URL, "token-1", "", ""))
	nodes, err := resolver.ResolveUserNodes(context.Background(), "charley")
	if err != nil {
		t.Fatalf("ResolveUserNodes returned error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].UUID != "uuid-1" || nodes[0].RealityPublicKey != "pk-1" {
		t.Fatalf("unexpected node: %#v", nodes[0])
	}
}

func TestResolverSearchUsersBuildsUniqueSummaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":[{"id":1,"remark":"vless","enable":true,"listen":"127.0.0.1","port":6443,"protocol":"vless","settings":"{\"clients\":[{\"email\":\"charley\",\"id\":\"uuid-1\",\"enable\":true},{\"email\":\"charlotte\",\"id\":\"uuid-2\",\"enable\":true}]}","streamSettings":"{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{\"settings\":{\"publicKey\":\"pk-1\",\"fingerprint\":\"chrome\",\"serverName\":\"www.cloudflare.com\"},\"shortIds\":[\"sid-1\"]}}"}]}`))
	}))
	defer server.Close()

	resolver := NewResolver(NewClient(server.URL, "token-1", "", ""))
	resolver.Client.HTTP = server.Client()

	users, err := resolver.SearchUsers(context.Background(), "char")
	if err != nil {
		t.Fatalf("SearchUsers returned error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Email == users[1].Email {
		t.Fatalf("expected unique users, got %#v", users)
	}
}
