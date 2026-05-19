//go:build integration

package cli_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func _hasOrchestrator() bool {
	return os.Getenv("ORCHESTRATOR_URL") != ""
}

// TestAskCmd_EndToEnd sends a known question to a running orchestrator and
// asserts the answer is non-empty. Requires ORCHESTRATOR_URL to be set and
// a fixture repo already ingested into Neo4j.
func TestAskCmd_EndToEnd(t *testing.T) {
	if !_hasOrchestrator() {
		t.Skip("ORCHESTRATOR_URL not set — skipping end-to-end test")
	}

	stdout, stderr, err := execute("ask", "Where is the payment handler?")

	require.NoError(t, err, "ask command must not return an error")
	assert.Empty(t, stderr, "no errors expected")
	assert.NotEmpty(t, stdout, "answer must be non-empty")
	assert.Contains(t, stdout, "##", "answer must include a Markdown heading")
}
