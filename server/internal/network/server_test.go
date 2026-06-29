package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aoi-demo/server/internal/config"
	"aoi-demo/server/internal/world"
)

func TestServiceHealthAndReadinessEndpoints(t *testing.T) {
	server := newTestServer(t)
	handler := server.routes()

	healthResp := httptest.NewRecorder()
	handler.ServeHTTP(healthResp, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthResp.Code, http.StatusOK)
	}

	var health healthReport
	decodeResponse(t, healthResp, &health)
	if health.Status != "ok" {
		t.Fatalf("health status field = %q, want ok", health.Status)
	}
	if health.Service != serviceName {
		t.Fatalf("health service = %q, want %q", health.Service, serviceName)
	}

	readyResp := httptest.NewRecorder()
	handler.ServeHTTP(readyResp, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d", readyResp.Code, http.StatusOK)
	}

	var ready readinessReport
	decodeResponse(t, readyResp, &ready)
	if ready.Status != "ready" {
		t.Fatalf("ready status field = %q, want ready", ready.Status)
	}
	if ready.Dependencies == nil {
		t.Fatal("ready dependencies should be an empty array, not null")
	}

	legacyResp := httptest.NewRecorder()
	handler.ServeHTTP(legacyResp, httptest.NewRequest(http.MethodGet, "/health", nil))
	if legacyResp.Code != http.StatusOK {
		t.Fatalf("legacy health status = %d, want %d", legacyResp.Code, http.StatusOK)
	}
	var legacy map[string]bool
	decodeResponse(t, legacyResp, &legacy)
	if !legacy["ok"] {
		t.Fatal("legacy health response should keep ok=true")
	}
}

func TestServiceVersionEndpoint(t *testing.T) {
	server := newTestServer(t)
	handler := server.routes()

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("version status = %d, want %d", resp.Code, http.StatusOK)
	}

	var version versionReport
	decodeResponse(t, resp, &version)
	if version.Service != serviceName {
		t.Fatalf("version service = %q, want %q", version.Service, serviceName)
	}
	if version.Version != "test-version" {
		t.Fatalf("version = %q, want test-version", version.Version)
	}
	if version.Commit != "test-commit" {
		t.Fatalf("commit = %q, want test-commit", version.Commit)
	}
	if version.ConfigPath != "/tmp/test-dev.yaml" {
		t.Fatalf("config path = %q, want /tmp/test-dev.yaml", version.ConfigPath)
	}
	if version.ListenAddr != "127.0.0.1:0" {
		t.Fatalf("listen addr = %q, want 127.0.0.1:0", version.ListenAddr)
	}
	if version.AOI["type"] != "grid" {
		t.Fatalf("aoi type = %v, want grid", version.AOI["type"])
	}
	if version.Sync["type"] != "snapshot" {
		t.Fatalf("sync type = %v, want snapshot", version.Sync["type"])
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Server.ListenAddr = "127.0.0.1:0"
	cfg.World.NPCCount = 0

	w, err := world.New(cfg)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}

	return NewServerWithInfo(cfg, w, ServiceInfo{
		Version:    "test-version",
		Commit:     "test-commit",
		BuildTime:  "test-build-time",
		ConfigPath: "/tmp/test-dev.yaml",
	})
}

func decodeResponse(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()

	if err := json.Unmarshal(resp.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response %q: %v", resp.Body.String(), err)
	}
}
