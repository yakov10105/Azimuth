package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/azimuth/azimuth/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execute runs the root command with the given args, capturing stdout and
// stderr separately. Returns the error Cobra/RunE produced, if any.
func execute(args ...string) (stdout, stderr string, err error) {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	cmd := cli.NewRootCmd()
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// ── root ────────────────────────────────────────────────────────────────────

func TestRootCmd_Help(t *testing.T) {
	stdout, _, err := execute("--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "zm")
	assert.Contains(t, stdout, "ingest")
	assert.Contains(t, stdout, "ask")
	assert.Contains(t, stdout, "status")
}

func TestRootCmd_Version(t *testing.T) {
	stdout, _, err := execute("--version")
	require.NoError(t, err)
	assert.Contains(t, stdout, "zm")
	assert.Contains(t, stdout, cli.Version)
}

func TestRootCmd_NoSubcommand_ShowsHelp(t *testing.T) {
	stdout, _, _ := execute()
	assert.Contains(t, stdout, "Usage:")
}

// ── ingest ──────────────────────────────────────────────────────────────────

func TestIngestCmd_Help(t *testing.T) {
	stdout, _, err := execute("ingest", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ingest <repo-path>")
	assert.Contains(t, stdout, "--dry-run")
	assert.Contains(t, stdout, "--language")
}

func TestIngestCmd_NoArgs_ReturnsError(t *testing.T) {
	_, _, err := execute("ingest")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "accepts 1 arg") || strings.Contains(err.Error(), "arg(s)"),
		"expected arg-count error, got: %v", err)
}

func TestIngestCmd_TooManyArgs_ReturnsError(t *testing.T) {
	_, _, err := execute("ingest", "path/a", "path/b")
	assert.Error(t, err)
}

func TestIngestCmd_InvalidPath_ReturnsUserError(t *testing.T) {
	_, _, err := execute("ingest", "/nonexistent/path/xyz")
	// Expect a user error (exit 1) — not a valid directory.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid directory")
}

// ── ask ─────────────────────────────────────────────────────────────────────

func TestAskCmd_Help(t *testing.T) {
	stdout, _, err := execute("ask", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ask")
	assert.Contains(t, stdout, "--json")
	assert.Contains(t, stdout, "--verbose")
	assert.Contains(t, stdout, "--diagram")
	assert.Contains(t, stdout, "--depth")
}

func TestAskCmd_NoArgs_ReturnsError(t *testing.T) {
	_, _, err := execute("ask")
	assert.Error(t, err)
}

func TestAskCmd_TooManyArgs_ReturnsError(t *testing.T) {
	_, _, err := execute("ask", "question one", "question two")
	assert.Error(t, err)
}

func TestAskCmd_StubOutput_ToStdout(t *testing.T) {
	stdout, stderr, err := execute("ask", "Where is the payment handler?")
	require.NoError(t, err)
	assert.Contains(t, stdout, "not yet implemented")
	assert.Empty(t, stderr, "stub must not write to stderr")
}

// ── status ───────────────────────────────────────────────────────────────────

func TestStatusCmd_Help(t *testing.T) {
	stdout, _, err := execute("status", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "status")
}

func TestStatusCmd_ExtraArgs_ReturnsError(t *testing.T) {
	_, _, err := execute("status", "unexpected")
	assert.Error(t, err)
}

func TestStatusCmd_StubOutput_ToStdout(t *testing.T) {
	stdout, stderr, err := execute("status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "not yet implemented")
	assert.Empty(t, stderr, "stub must not write to stderr")
}
