package acceptance

import "testing"

// TestBootstrapSmoke is a throwaway end-to-end proof that Bootstrap works
// against the running stack: register -> confirm via Mailcatcher -> mint
// an API key. Not part of the acceptance-test suite proper (no TF_ACC
// gate) since it exists only to verify the harness itself.
func TestBootstrapSmoke(t *testing.T) {
	endpoint, apiKey := Bootstrap(t)
	if endpoint == "" {
		t.Fatal("expected non-empty endpoint")
	}
	if apiKey == "" {
		t.Fatal("expected non-empty apiKey")
	}
	t.Logf("bootstrap ok: endpoint=%s apiKey=%s...", endpoint, apiKey[:min(8, len(apiKey))])
}
