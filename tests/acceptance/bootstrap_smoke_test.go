package acceptance

import (
	"os"
	"testing"
)

// TestBootstrapSmoke is a throwaway end-to-end proof that Bootstrap works
// against the running stack: register -> confirm via Mailcatcher -> mint
// an API key. It exists only to verify the harness itself (not a stand-in
// for Task 6's real acceptance tests), but is gated behind TF_ACC like
// every other acceptance test in this package so a plain `go test ./...`
// without a running stack doesn't fail.
func TestBootstrapSmoke(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test skipped: set TF_ACC=1 to run against tests/acceptance's docker compose stack")
	}

	endpoint, apiKey := Bootstrap(t)
	if endpoint == "" {
		t.Fatal("expected non-empty endpoint")
	}
	if apiKey == "" {
		t.Fatal("expected non-empty apiKey")
	}
	t.Logf("bootstrap ok: endpoint=%s apiKey=%s...", endpoint, apiKey[:min(8, len(apiKey))])
}
