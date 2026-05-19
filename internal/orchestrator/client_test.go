package orchestrator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/azimuth/azimuth/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// healthyServer returns a test server with /healthz → 200 and /ask → validResp.
func healthyServer(t *testing.T, askResp orchestrator.AskResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/ask":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(askResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

var defaultAskResp = orchestrator.AskResponse{
	Summary:         "HandlePayment is in payments/handler.go",
	CallPath:        []string{"HandlePayment (payments/handler.go:38)"},
	RelevantFiles:   []string{"payments/handler.go"},
	EntryPointCount: 1,
	GraphNodeCount:  3,
}

// ── Ping ─────────────────────────────────────────────────────────────────────

func TestClient_Ping_OK(t *testing.T) {
	srv := healthyServer(t, defaultAskResp)
	defer srv.Close()

	c := orchestrator.NewClient(srv.URL)
	err := c.Ping(context.Background())
	require.NoError(t, err)
}

func TestClient_Ping_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := orchestrator.NewClient(srv.URL)
	err := c.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestClient_Ping_Unreachable(t *testing.T) {
	// Use a server that's immediately closed so the port is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := orchestrator.NewClient(srv.URL)
	err := c.Ping(context.Background())
	require.Error(t, err)
}

// ── Ask ──────────────────────────────────────────────────────────────────────

func TestClient_Ask_OK(t *testing.T) {
	srv := healthyServer(t, defaultAskResp)
	defer srv.Close()

	c := orchestrator.NewClient(srv.URL)
	resp, err := c.Ask(context.Background(), orchestrator.AskRequest{Question: "Where is X?", Depth: 3})
	require.NoError(t, err)
	assert.Equal(t, defaultAskResp.Summary, resp.Summary)
	assert.Equal(t, defaultAskResp.CallPath, resp.CallPath)
	assert.Equal(t, defaultAskResp.RelevantFiles, resp.RelevantFiles)
	assert.Equal(t, 1, resp.EntryPointCount)
	assert.Equal(t, 3, resp.GraphNodeCount)
}

func TestClient_Ask_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ask" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := orchestrator.NewClient(srv.URL)
	_, err := c.Ask(context.Background(), orchestrator.AskRequest{Question: "test", Depth: 3})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Ask_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	c := orchestrator.NewClient(srv.URL)
	_, err := c.Ask(context.Background(), orchestrator.AskRequest{Question: "test", Depth: 3})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}
