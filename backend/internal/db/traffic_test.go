package db

import (
	"fmt"
	"testing"
)

func TestZeroUserTrafficForServerOnlyClearsMatchingRows(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := store.ReplaceUserTraffic([]UserTraffic{
		{Email: "alice@example.com", ServerID: 1, Up: 10, Down: 20},
		{Email: "bob@example.com", ServerID: 1, Up: 30, Down: 40},
		{Email: "alice@example.com", ServerID: 2, Up: 50, Down: 60},
	}); err != nil {
		t.Fatalf("ReplaceUserTraffic returned error: %v", err)
	}

	if err := store.ZeroUserTrafficForServer(1, []string{"alice@example.com"}); err != nil {
		t.Fatalf("ZeroUserTrafficForServer returned error: %v", err)
	}

	all, err := store.GetAllUserTraffic()
	if err != nil {
		t.Fatalf("GetAllUserTraffic returned error: %v", err)
	}

	got := map[string]UserTraffic{}
	for _, entry := range all {
		got[fmt.Sprintf("%s#%d", entry.Email, entry.ServerID)] = entry
	}

	if entry := got["alice@example.com#1"]; entry.Up != 0 || entry.Down != 0 {
		t.Fatalf("expected server 1 traffic to be zeroed, got %#v", entry)
	}
	if entry := got["bob@example.com#1"]; entry.Up != 30 || entry.Down != 40 {
		t.Fatalf("expected unrelated server 1 user to remain unchanged, got %#v", entry)
	}
	if entry := got["alice@example.com#2"]; entry.Up != 50 || entry.Down != 60 {
		t.Fatalf("expected other server traffic to remain unchanged, got %#v", entry)
	}
}

func TestReplaceUserTrafficForServerOnlyTouchesOneServer(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := store.ReplaceUserTraffic([]UserTraffic{
		{Email: "alice@example.com", ServerID: 1, Up: 10, Down: 20},
		{Email: "bob@example.com", ServerID: 2, Up: 30, Down: 40},
	}); err != nil {
		t.Fatalf("ReplaceUserTraffic returned error: %v", err)
	}

	if err := store.ReplaceUserTrafficForServer(1, []UserTraffic{
		{Email: "alice@example.com", ServerID: 1, Up: 0, Down: 5},
		{Email: "carol@example.com", ServerID: 1, Up: 6, Down: 7},
	}); err != nil {
		t.Fatalf("ReplaceUserTrafficForServer returned error: %v", err)
	}

	all, err := store.GetAllUserTraffic()
	if err != nil {
		t.Fatalf("GetAllUserTraffic returned error: %v", err)
	}
	got := map[string]UserTraffic{}
	for _, entry := range all {
		got[fmt.Sprintf("%s#%d", entry.Email, entry.ServerID)] = entry
	}

	if _, ok := got["bob@example.com#2"]; !ok {
		t.Fatalf("expected other server row to remain, got %#v", got)
	}
	if entry := got["alice@example.com#1"]; entry.Up != 0 || entry.Down != 5 {
		t.Fatalf("expected server 1 row to be replaced, got %#v", entry)
	}
	if entry := got["carol@example.com#1"]; entry.Up != 6 || entry.Down != 7 {
		t.Fatalf("expected new server 1 row to be inserted, got %#v", entry)
	}
}
