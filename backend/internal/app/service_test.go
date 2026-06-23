package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"subforme/backend/internal/config"
	"subforme/backend/internal/db"
	"subforme/backend/internal/xui"
)

func TestGenerateReturnsErrorWhenUserMissing(t *testing.T) {
	svc := Service{}
	_, err := svc.Generate("charley")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsertClientInSettingsPreservesInboundSettings(t *testing.T) {
	raw := `{"clients":[{"email":"alice","id":"uuid-1"}],"decryption":"none","testseed":[900,500]}`
	got := upsertClientInSettings(raw, xui.InboundClient{Email: "bob", ID: "uuid-2", Enable: true})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("settings json is invalid: %v", err)
	}
	if decoded["decryption"] != "none" {
		t.Fatalf("expected decryption to be preserved, got %#v", decoded)
	}
	clients, ok := decoded["clients"].([]any)
	if !ok || len(clients) != 2 {
		t.Fatalf("expected two clients, got %#v", decoded["clients"])
	}
}

func TestSameRemoteClientIgnoresEnable(t *testing.T) {
	existing := xui.ClientListRecord{Email: "alice", UUID: "uuid-1", Password: "pass", Auth: "auth", Flow: "flow", SubID: "sub", Enable: false}
	desired := xui.InboundClient{Email: "alice", ID: "uuid-1", Password: "pass", Auth: "auth", Flow: "flow", SubID: "sub", Enable: true}
	if !sameRemoteClient(existing, desired) {
		t.Fatal("expected enable mismatch to be ignored")
	}
}

func TestDesiredInboundTagsIncludesDisabledAssignments(t *testing.T) {
	assignments := []db.UserAssignment{{
		EmailOnServer: "alice",
		InboundID:     10,
		Enable:        false,
	}}
	inbounds := map[int64]db.Inbound{
		10: {Protocol: "vless", Listen: "", Port: 443, Remark: "HK"},
	}

	got := desiredInboundTagsByEmail(assignments, inbounds)
	if len(got["alice"]) != 1 {
		t.Fatalf("expected disabled assignment to remain sync-desired, got %#v", got)
	}
}

func TestBuildXUIClientDoesNotPropagateDisabledState(t *testing.T) {
	got := buildXUIClient("vless", db.User{
		Email:  "alice",
		UUID:   "uuid-1",
		Enable: false,
	})
	if !got.Enable {
		t.Fatal("expected synced client payload to stay enabled by default")
	}
}

func TestApplyManagedNodesMatchesProtocolAndNetwork(t *testing.T) {
	rawReality := xui.Node{
		Name:             "raw",
		Type:             "vless",
		Server:           "panel.example.com",
		Port:             443,
		Network:          "raw",
		TLS:              true,
		RealityPublicKey: "pk-1",
		RealityShortID:   "sid-1",
		ServerID:         1,
	}
	xhttpTLS := xui.Node{
		Name:     "xhttp",
		Type:     "vless",
		Server:   "panel.example.com",
		Port:     443,
		Network:  "xhttp",
		TLS:      true,
		ServerID: 1,
	}
	got := applyManagedNodes(
		[]xui.Node{rawReality, xhttpTLS},
		[]config.ManagedNode{{
			ID:       "hk-xhttp",
			Name:     "hk-xhttp",
			Address:  "www.4738.org",
			Port:     443,
			Protocol: "vless",
			Network:  "xhttp",
			Flow:     "xtls-rprx-vision",
			ServerID: 1,
		}},
		[]string{"hk-xhttp"},
	)
	if len(got) != 1 {
		t.Fatalf("expected one managed node, got %#v", got)
	}
	if got[0].Network != "xhttp" || got[0].Flow != "" || got[0].RealityPublicKey != "" || got[0].Server != "www.4738.org" {
		t.Fatalf("expected managed xhttp node to use xhttp template, got %#v", got[0])
	}
}

func TestApplyManagedNodesPreservesTemplateFlowWhenManagedFlowIsEmpty(t *testing.T) {
	template := xui.Node{
		Name:     "hk",
		Type:     "vless",
		Server:   "panel.example.com",
		Port:     443,
		Network:  "raw",
		TLS:      true,
		Flow:     "xtls-rprx-vision",
		ServerID: 1,
	}

	got := applyManagedNodes(
		[]xui.Node{template},
		[]config.ManagedNode{{
			ID:       "hk",
			Name:     "hk",
			Address:  "hk.example.com",
			Port:     443,
			Protocol: "vless",
			Network:  "raw",
			Flow:     "",
			ServerID: 1,
		}},
		[]string{"hk"},
	)
	if len(got) != 1 {
		t.Fatalf("expected one managed node, got %#v", got)
	}
	if got[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("expected empty managed flow to preserve template flow, got %#v", got[0])
	}
}

func TestApplyImportedClientDoesNotClearFlowFromEmptyInbound(t *testing.T) {
	user := &db.User{
		Email:    "alice",
		UUID:     "uuid-1",
		Flow:     "xtls-rprx-vision",
		Security: "auto",
	}

	changed := applyImportedClient(user, xui.InboundClient{
		Email: "alice",
		ID:    "uuid-1",
	})
	if changed {
		t.Fatalf("expected empty imported flow/security to be ignored, got %#v", user)
	}
	if user.Flow != "xtls-rprx-vision" || user.Security != "auto" {
		t.Fatalf("expected flow/security to remain, got %#v", user)
	}
}

func TestApplyManagedNodesFallsBackByProtocolWithoutLeakingReality(t *testing.T) {
	rawReality := xui.Node{
		Name:             "raw",
		Type:             "vless",
		Server:           "panel.example.com",
		Port:             443,
		Network:          "raw",
		TLS:              true,
		RealityPublicKey: "pk-1",
		RealityShortID:   "sid-1",
		ServerID:         1,
	}

	got := applyManagedNodes(
		[]xui.Node{rawReality},
		[]config.ManagedNode{{
			ID:       "hk-xhttp",
			Name:     "hk-xhttp",
			Address:  "www.4738.org",
			Port:     443,
			Protocol: "vless",
			Network:  "xhttp",
			ServerID: 1,
		}},
		[]string{"hk-xhttp"},
	)
	if len(got) != 1 {
		t.Fatalf("expected one fallback managed node, got %#v", got)
	}
	if got[0].Network != "xhttp" || got[0].RealityPublicKey != "" || got[0].RealityShortID != "" {
		t.Fatalf("expected fallback to adapt network and clear reality opts, got %#v", got[0])
	}
	if got[0].XHTTPPath != "" || got[0].Flow != "" || !got[0].TLS {
		t.Fatalf("expected fallback xhttp node to keep xhttp skeleton only, got %#v", got[0])
	}
}

func TestApplyManagedNodesFallsBackAcrossServers(t *testing.T) {
	template := xui.Node{
		Name:     "hk",
		Type:     "vless",
		Server:   "hk.4738.org",
		Port:     443,
		Network:  "raw",
		TLS:      true,
		ServerID: 3,
	}
	got := applyManagedNodes(
		[]xui.Node{template},
		[]config.ManagedNode{
			{ID: "bwg", Name: "bwg", Address: "bwg.4738.org", Port: 443, Protocol: "vless", Network: "raw", ServerID: 1},
			{ID: "dm", Name: "dm", Address: "dm.4738.org", Port: 443, Protocol: "vless", Network: "raw", ServerID: 2},
			{ID: "hk", Name: "hk", Address: "hk.4738.org", Port: 443, Protocol: "vless", Network: "raw", ServerID: 3},
		},
		[]string{"bwg", "dm", "hk"},
	)
	if len(got) != 3 {
		t.Fatalf("expected all managed nodes to be generated from fallback template, got %#v", got)
	}
	if got[0].Name != "bwg" || got[1].Name != "dm" || got[2].Name != "hk" {
		t.Fatalf("unexpected managed node names: %#v", got)
	}
}

func TestSyncServerInboundsReusesExistingPortWhenKeyChanged(t *testing.T) {
	store, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	var addCalled atomic.Bool
	var updateCalled atomic.Bool
	remotePayload := `{"success":true,"obj":[{"id":2,"remark":"xhttp","enable":true,"listen":"127.0.0.1","port":6444,"protocol":"vless","settings":"{\"clients\":[],\"decryption\":\"none\"}","streamSettings":"{\"network\":\"xhttp\"}","tag":"old-xhttp"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/inbounds/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(remotePayload))
		case "/panel/api/inbounds/add":
			addCalled.Store(true)
			http.Error(w, "add should not be called", http.StatusConflict)
		case "/panel/api/inbounds/update/2":
			updateCalled.Store(true)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := Service{DB: store}
	cli := xui.NewClient(server.URL, "token-1", "", "")
	remote := []xui.InboundRecord{{
		ID:             2,
		Remark:         "xhttp",
		Enable:         true,
		Listen:         "127.0.0.1",
		Port:           6444,
		Protocol:       "vless",
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"xhttp"}`,
		Tag:            "old-xhttp",
	}}
	main := []db.Inbound{{
		InboundID:          7,
		Remark:             "xhttp",
		Enable:             true,
		Listen:             "127.0.0.1",
		Port:               6444,
		Protocol:           "vless",
		SettingsJSON:       `{"clients":[],"decryption":"none"}`,
		StreamSettingsJSON: `{"network":"xhttp"}`,
		Tag:                "main-xhttp",
	}}

	if _, err := svc.syncServerInbounds(context.Background(), cli, db.Server{ID: 1, Name: "bwg"}, main, remote); err != nil {
		t.Fatalf("syncServerInbounds returned error: %v", err)
	}
	if addCalled.Load() {
		t.Fatal("expected sync to reuse existing port instead of adding inbound")
	}
	if !updateCalled.Load() {
		t.Fatal("expected sync to update existing inbound with matching listen/port")
	}
}

func TestSyncServerInboundClientsUsesMainInboundClientPayload(t *testing.T) {
	var updates []xui.InboundRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/inbounds/update/11", "/panel/api/inbounds/update/12":
			var payload xui.InboundRecord
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updates = append(updates, payload)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cli := xui.NewClient(server.URL, "token-1", "", "")
	remote := []db.Inbound{
		{
			InboundID:    11,
			Remark:       "vless",
			Listen:       "127.0.0.1",
			Port:         6443,
			Protocol:     "vless",
			SettingsJSON: `{"clients":[{"email":"charley","id":"uuid-1","flow":"","enable":true}],"decryption":"none"}`,
			Tag:          "reality-tag",
			Enable:       true,
		},
		{
			InboundID:    12,
			Remark:       "xhttp",
			Listen:       "127.0.0.1",
			Port:         6444,
			Protocol:     "vless",
			SettingsJSON: `{"clients":[{"email":"charley","id":"uuid-1","flow":"xtls-rprx-vision","enable":true}],"decryption":"none"}`,
			Tag:          "xhttp-tag",
			Enable:       true,
		},
	}
	desired := map[string][]xui.InboundClient{
		inboundSyncKey(remote[0]): {{Email: "charley", ID: "uuid-1", Flow: "xtls-rprx-vision", Enable: true}},
		inboundSyncKey(remote[1]): {{Email: "charley", ID: "uuid-1", Flow: "", Enable: true}},
	}
	if len(desired[inboundSyncKey(remote[0])]) != 1 || len(desired[inboundSyncKey(remote[1])]) != 1 {
		t.Fatalf("expected desired clients to be keyed by inbound tag, got %#v", desired)
	}
	if inboundSyncKey(remote[0]) != "reality-tag" || inboundSyncKey(remote[1]) != "xhttp-tag" {
		t.Fatalf("unexpected inbound sync keys: %q %q", inboundSyncKey(remote[0]), inboundSyncKey(remote[1]))
	}
	checkSettings := replaceClientsInSettings(remote[0].SettingsJSON, desired[inboundSyncKey(remote[0])])
	checkParsed, ok := xui.ParseInboundSettings(checkSettings)
	if !ok || len(checkParsed.Clients) != 1 || checkParsed.Clients[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("expected replaceClientsInSettings to preserve desired client flow, got %q", checkSettings)
	}

	syncedUsers, updatedInbounds, deletedClients, err := syncServerInboundClients(context.Background(), cli, remote, desired)
	if err != nil {
		t.Fatalf("syncServerInboundClients returned error: %v", err)
	}
	if syncedUsers != 2 || updatedInbounds != 2 || deletedClients != 0 {
		t.Fatalf("unexpected counters: synced=%d updated=%d deleted=%d", syncedUsers, updatedInbounds, deletedClients)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 inbound updates, got %d", len(updates))
	}
	got := map[int]string{}
	for _, update := range updates {
		settings, ok := xui.ParseInboundSettings(update.Settings)
		if !ok || len(settings.Clients) != 1 {
			t.Fatalf("expected one synced client in settings, got %#v", update)
		}
		got[update.Port] = settings.Clients[0].Flow
	}
	if got[6443] != "xtls-rprx-vision" {
		t.Fatalf("expected reality inbound flow to be restored, got %#v", got)
	}
	if got[6444] != "" {
		t.Fatalf("expected xhttp inbound flow to stay empty, got %#v", got)
	}
}

func TestPreserveRemoteClientEnableKeepsSubPanelStatus(t *testing.T) {
	desired := []xui.InboundClient{
		{Email: "charley", ID: "uuid-1", Flow: "xtls-rprx-vision", Enable: false},
		{Email: "alice", ID: "uuid-2", Flow: "", Enable: true},
	}
	current := []xui.InboundClient{
		{Email: "charley", ID: "uuid-1", Enable: true},
	}

	got := preserveRemoteClientEnable(desired, current)
	if len(got) != 2 {
		t.Fatalf("expected 2 clients, got %#v", got)
	}
	if !got[0].Enable {
		t.Fatalf("expected existing sub-panel enable=true to be preserved, got %#v", got[0])
	}
	if !got[1].Enable {
		t.Fatalf("expected new client to keep desired default enable, got %#v", got[1])
	}
}

func TestRemoveClientFromSettingsPreservesInboundSettings(t *testing.T) {
	raw := `{"clients":[{"email":"alice","id":"uuid-1"},{"email":"bob","id":"uuid-2"}],"encryption":"none"}`
	got := removeClientFromSettings(raw, "alice")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("settings json is invalid: %v", err)
	}
	if decoded["encryption"] != "none" {
		t.Fatalf("expected encryption to be preserved, got %#v", decoded)
	}
	clients, ok := decoded["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("expected one remaining client, got %#v", decoded["clients"])
	}
	remaining := clients[0].(map[string]any)
	if remaining["email"] != "bob" {
		t.Fatalf("expected bob to remain, got %#v", remaining)
	}
}

func TestGenerateBuildsYAMLFromBundleAndNodes(t *testing.T) {
	svc := Service{
		ConfigDir: "ignored",
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{
					Mode: "whitelist",
					XUI:  config.XUIConfig{BaseURL: "https://panel.example.com/xui", APIKey: "token"},
				},
				BaseWhitelist: config.BaseConfig{
					Mode:        "rule",
					LogLevel:    "info",
					Proxies:     []any{},
					ProxyGroups: []any{},
					Rules:       []string{"MATCH,DIRECT"},
				},
				BaseBlacklist: config.BaseConfig{
					Mode:        "rule",
					LogLevel:    "info",
					Proxies:     []any{},
					ProxyGroups: []any{},
					Rules:       []string{"MATCH,PROXY"},
				},
				Groups: config.GroupConfig{
					Regions: map[string][]string{
						"HK": {"HK"},
					},
					GroupNames: config.GroupNames{
						Proxy: "节点选择",
						Auto:  "自动选择",
						Other: "其他节点",
					},
					Healthcheck: config.GroupHealthcheck{
						URL:             "https://www.gstatic.com/generate_204",
						IntervalSeconds: 300,
					},
				},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{
				nodes: []xui.Node{
					{
						Name:              "HK-01",
						Type:              "vless",
						Server:            "hk.example.com",
						Port:              443,
						UUID:              "uuid-1",
						TLS:               true,
						ClientFingerprint: "chrome",
					},
				},
			}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			return "mixed-port: 10801\nallow-lan: true\nmode: rule\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	raw, err := svc.Generate("charley")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, "name: HK-01") {
		t.Fatalf("expected generated proxy in yaml, got %s", got)
	}
	if !strings.Contains(got, "name: 节点选择") {
		t.Fatalf("expected generated group in yaml, got %s", got)
	}
}

func TestGenerateUsesFirstSelectGroupWhenGroupNameProxyIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveProviders(tmp, []config.ProviderAddon{{
		ID:             "airport",
		Name:           "airport",
		ProxyProviders: map[string]any{"airport": map[string]any{"type": "http", "url": "https://example.com/provider.yaml"}},
	}}); err != nil {
		t.Fatalf("save providers: %v", err)
	}

	svc := Service{
		ConfigDir: tmp,
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{
					Mode:          "whitelist",
					UserProviders: map[string][]string{"alice": {"airport"}},
				},
				BaseWhitelist: config.BaseConfig{
					Rules: []string{"MATCH,DIRECT"},
				},
				BaseBlacklist: config.BaseConfig{
					Rules: []string{"MATCH,PROXY"},
				},
				Groups: config.GroupConfig{
					Groups: []config.GroupDef{
						{Name: "PROXY", Type: "select"},
						{Name: "GPT", Type: "url-test", URL: "https://www.gstatic.com/generate_204", Interval: 300},
					},
				},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{
				nodes: []xui.Node{{Name: "node-a", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid"}},
			}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			return "proxies: []\nproxy-groups: []\nproxy-providers: {}\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	raw, err := svc.Generate("alice")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "name: PROXY") || !strings.Contains(got, "- airport") {
		t.Fatalf("expected provider to be attached to PROXY, got %s", got)
	}
}

func TestGenerateWithBaseURLRewritesProviderURL(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveProviders(tmp, []config.ProviderAddon{{
		ID:   "airport",
		Name: "airport",
		ProxyProviders: map[string]any{"airport": map[string]any{
			"type": "http",
			"url":  "http://127.0.0.1:8080/api/proxy-providers/airport.yaml",
		}},
	}}); err != nil {
		t.Fatalf("save providers: %v", err)
	}

	svc := Service{
		ConfigDir: tmp,
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{
					Mode:          "whitelist",
					UserProviders: map[string][]string{"alice": {"airport"}},
				},
				BaseWhitelist: config.BaseConfig{Rules: []string{"MATCH,DIRECT"}},
				BaseBlacklist: config.BaseConfig{Rules: []string{"MATCH,PROXY"}},
				Groups: config.GroupConfig{Groups: []config.GroupDef{
					{Name: "PROXY", Type: "select"},
				}},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{nodes: []xui.Node{{Name: "node-a", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid"}}}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			return "proxies: []\nproxy-groups: []\nproxy-providers: {}\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	raw, err := svc.GenerateWithBaseURL("alice", "https://sub.4738.org")
	if err != nil {
		t.Fatalf("GenerateWithBaseURL returned error: %v", err)
	}
	got := string(raw)
	if strings.Contains(got, "127.0.0.1") {
		t.Fatalf("provider URL still contains localhost: %s", got)
	}
	if !strings.Contains(got, "https://sub.4738.org/api/proxy-providers/airport.yaml") {
		t.Fatalf("provider URL was not rewritten: %s", got)
	}
}

func TestGenerateWithVariantUsesIOSTemplate(t *testing.T) {
	var loadedMode string
	svc := Service{
		ConfigDir: "ignored",
		Loader: func(dir string) (config.Bundle, error) {
			return config.Bundle{
				App: config.AppConfig{Mode: "whitelist"},
				Groups: config.GroupConfig{Groups: []config.GroupDef{
					{Name: "PROXY", Type: "select"},
				}},
			}, nil
		},
		ResolverFactory: func(cfg config.XUIConfig) XUIResolver {
			return fakeResolver{nodes: []xui.Node{{Name: "node-a", Type: "vless", Server: "example.com", Port: 443, UUID: "uuid"}}}
		},
		TemplateLoader: func(dir, mode string) (string, error) {
			loadedMode = mode
			return "proxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n", nil
		},
	}

	if _, err := svc.GenerateWithBaseURLAndVariant("alice", "", "ios"); err != nil {
		t.Fatalf("GenerateWithBaseURLAndVariant returned error: %v", err)
	}
	if loadedMode != "ios" {
		t.Fatalf("expected ios template, got %q", loadedMode)
	}
}

func TestBuildNodeFromCacheIncludesXHTTPSettings(t *testing.T) {
	node := buildNodeFromCache(
		db.Inbound{
			Remark:             "xhttp",
			Protocol:           "vless",
			Port:               6444,
			SettingsJSON:       `{"clients":[{"email":"charley","id":"036898f6-8f62-46b2-9924-c1b884d6e75d","flow":"","security":"","enable":true}]}`,
			StreamSettingsJSON: `{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"xhttp.example.com","fingerprint":"chrome"},"realitySettings":{"shortIds":["stale"],"settings":{"publicKey":"stale-pk","fingerprint":"stale-fp","serverName":"stale.example.com"}},"xhttpSettings":{"path":"/xhttp","host":"","mode":"auto"}}`,
		},
		db.User{
			Email: "charley",
			UUID:  "036898f6-8f62-46b2-9924-c1b884d6e75d",
			Flow:  "xtls-rprx-vision",
		},
		"panel.example.com",
	)

	if node.Network != "xhttp" {
		t.Fatalf("expected xhttp network, got %#v", node)
	}
	if node.XHTTPPath != "/xhttp" || node.XHTTPMode != "auto" {
		t.Fatalf("expected xhttp settings to survive cache build, got %#v", node)
	}
	if node.RealityPublicKey != "" || node.RealityShortID != "" || node.ServerName != "xhttp.example.com" {
		t.Fatalf("expected non-reality xhttp to ignore reality settings, got %#v", node)
	}
	if node.Flow != "" {
		t.Fatalf("expected xhttp node flow to come from inbound client and stay empty, got %#v", node)
	}
}

func TestBuildNodeFromCacheUsesInboundClientFlow(t *testing.T) {
	node := buildNodeFromCache(
		db.Inbound{
			Remark:             "reality",
			Protocol:           "vless",
			Port:               443,
			SettingsJSON:       `{"clients":[{"email":"charley","id":"036898f6-8f62-46b2-9924-c1b884d6e75d","flow":"xtls-rprx-vision","security":"auto","enable":true}]}`,
			StreamSettingsJSON: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.cloudflare.com"],"shortIds":["sid-1"],"settings":{"publicKey":"pk-1","fingerprint":"chrome","serverName":"www.cloudflare.com"}}}`,
		},
		db.User{
			Email:    "charley",
			UUID:     "036898f6-8f62-46b2-9924-c1b884d6e75d",
			Flow:     "",
			Security: "",
		},
		"panel.example.com",
	)

	if node.Flow != "xtls-rprx-vision" || node.Security != "auto" {
		t.Fatalf("expected node flow/security from inbound client, got %#v", node)
	}
}

func TestBuildNodeFromCacheClearsXHTTPFieldsForRawVLESS(t *testing.T) {
	node := buildNodeFromCache(
		db.Inbound{
			Remark:             "reality",
			Protocol:           "vless",
			Port:               443,
			SettingsJSON:       `{"clients":[{"email":"charley","id":"036898f6-8f62-46b2-9924-c1b884d6e75d","flow":"xtls-rprx-vision","enable":true}]}`,
			StreamSettingsJSON: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.cloudflare.com"],"shortIds":["sid-1"],"settings":{"publicKey":"pk-1","fingerprint":"chrome","serverName":"www.cloudflare.com"}},"xhttpSettings":{"path":"/stale","host":"stale.example.com","mode":"packet-up"}}`,
		},
		db.User{
			Email: "charley",
			UUID:  "036898f6-8f62-46b2-9924-c1b884d6e75d",
		},
		"panel.example.com",
	)

	if node.Network != "raw" {
		t.Fatalf("expected raw network, got %#v", node)
	}
	if node.XHTTPPath != "" || node.XHTTPHost != "" || node.XHTTPMode != "" {
		t.Fatalf("expected raw vless node to clear xhttp settings, got %#v", node)
	}
	if node.Flow != "xtls-rprx-vision" || node.RealityPublicKey != "pk-1" {
		t.Fatalf("expected raw reality fields to remain, got %#v", node)
	}
}

func TestStartTrafficSchedulerLoopRefreshesImmediatelyAndOnInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		startTrafficSchedulerLoop(ctx, 10*time.Millisecond, func() {
			if calls.Add(1) >= 2 {
				cancel()
			}
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for traffic refresher loop")
	}

	if got := calls.Load(); got < 2 {
		t.Fatalf("expected at least 2 refresh calls, got %d", got)
	}
}

func TestShouldSyncServerTrafficUsesServerInterval(t *testing.T) {
	now := time.Now().Unix()
	sv := db.Server{TrafficSyncIntervalMinutes: 30, LastTrafficSyncAt: now - 29*60}
	if shouldSyncServerTraffic(now, sv) {
		t.Fatal("expected sync to wait until interval is reached")
	}
	sv.LastTrafficSyncAt = now - 30*60
	if !shouldSyncServerTraffic(now, sv) {
		t.Fatal("expected sync once interval is reached")
	}
}

func TestShouldResetServerTrafficUsesMonthlyKey(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 6, 3, 12, 5, 0, 0, loc)
	sv := db.Server{
		AutoResetTrafficEnabled: true,
		AutoResetDay:            3,
		AutoResetHour:           12,
		AutoResetMinute:         0,
		AutoResetTimezone:       "Asia/Shanghai",
	}
	if !shouldResetServerTraffic(now, sv) {
		t.Fatal("expected reset to be due once scheduled time has passed")
	}
	sv.LastTrafficResetKey = "2026-06"
	if shouldResetServerTraffic(now, sv) {
		t.Fatal("expected reset to run only once per month")
	}
}

type fakeResolver struct {
	nodes  []xui.Node
	users  []xui.UserSummary
	avail  []xui.AvailableNode
	status xui.ConnectionStatus
	err    error
}

func (f fakeResolver) ResolveUserNodes(ctx context.Context, query string) ([]xui.Node, error) {
	return f.nodes, f.err
}

func (f fakeResolver) SearchUsers(ctx context.Context, query string) ([]xui.UserSummary, error) {
	return f.users, f.err
}

func (f fakeResolver) ListAvailableNodes(ctx context.Context) ([]xui.AvailableNode, error) {
	return f.avail, f.err
}

func (f fakeResolver) TestConnection(ctx context.Context) (xui.ConnectionStatus, error) {
	return f.status, f.err
}
