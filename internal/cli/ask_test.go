package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/azimuth/azimuth/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrchestratorServer starts a test server that serves /healthz and /ask.
func mockOrchestratorServer(t *testing.T, resp orchestrator.AskResponse) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/ask":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

var paymentHandlerResp = orchestrator.AskResponse{
	Summary:         "HandlePayment is in payments/handler.go at line 38.",
	CallPath:        []string{"main.RegisterRoutes (main.go:12)", "payments.HandlePayment (payments/handler.go:38)"},
	RelevantFiles:   []string{"payments/handler.go", "main.go"},
	EntryPointCount: 1,
	GraphNodeCount:  4,
}

func TestAskCmd_MarkdownOutput(t *testing.T) {
	srv := mockOrchestratorServer(t, paymentHandlerResp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, stderr, err := execute("ask", "Where is the payment handler?")

	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "## Where is the payment handler?")
	assert.Contains(t, stdout, "HandlePayment is in payments/handler.go")
	assert.Contains(t, stdout, "**Call path:**")
	assert.Contains(t, stdout, "payments.HandlePayment")
	assert.Contains(t, stdout, "**Relevant files:**")
	assert.Contains(t, stdout, "payments/handler.go")
}

func TestAskCmd_JSONOutput(t *testing.T) {
	srv := mockOrchestratorServer(t, paymentHandlerResp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, stderr, err := execute("ask", "--json", "Where is the payment handler?")

	require.NoError(t, err)
	assert.Empty(t, stderr)

	var got orchestrator.AskResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &got), "stdout must be valid JSON")
	assert.Equal(t, paymentHandlerResp.Summary, got.Summary)
	assert.Equal(t, paymentHandlerResp.CallPath, got.CallPath)
	assert.Equal(t, paymentHandlerResp.RelevantFiles, got.RelevantFiles)
}

func TestAskCmd_VerboseOutput(t *testing.T) {
	srv := mockOrchestratorServer(t, paymentHandlerResp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, _, err := execute("ask", "--verbose", "Where is the payment handler?")

	require.NoError(t, err)
	assert.Contains(t, stdout, "Entry points found: 1")
	assert.Contains(t, stdout, "Graph bubble size: 4 nodes")
}

func TestAskCmd_DepthFlagPassedToServer(t *testing.T) {
	var capturedDepth int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/ask":
			var req struct {
				Depth int `json:"depth"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			capturedDepth = req.Depth
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(paymentHandlerResp)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	_, _, err := execute("ask", "--depth", "5", "Where is the payment handler?")
	require.NoError(t, err)
	assert.Equal(t, 5, capturedDepth)
}

func TestAskCmd_EmptyCallPath_NoCallPathSection(t *testing.T) {
	resp := orchestrator.AskResponse{
		Summary:       "No call path available.",
		CallPath:      []string{},
		RelevantFiles: []string{"payments/handler.go"},
	}
	srv := mockOrchestratorServer(t, resp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, _, err := execute("ask", "Where is the payment handler?")

	require.NoError(t, err)
	assert.NotContains(t, stdout, "**Call path:**")
	assert.Contains(t, stdout, "**Relevant files:**")
}

func TestAskCmd_EmptyRelevantFiles_NoRelevantFilesSection(t *testing.T) {
	resp := orchestrator.AskResponse{
		Summary:       "Answer without files.",
		CallPath:      []string{},
		RelevantFiles: []string{},
	}
	srv := mockOrchestratorServer(t, resp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, _, err := execute("ask", "test question?")

	require.NoError(t, err)
	assert.NotContains(t, stdout, "**Relevant files:**")
	assert.Contains(t, stdout, "## test question?")
	assert.Contains(t, stdout, "Answer without files.")
}

func TestAskCmd_JSONDoesNotContainMarkdownHeading(t *testing.T) {
	srv := mockOrchestratorServer(t, paymentHandlerResp)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	stdout, _, err := execute("ask", "--json", "Where is the payment handler?")

	require.NoError(t, err)
	assert.False(t, strings.Contains(stdout, "## "), "JSON output must not contain Markdown headings")
}
