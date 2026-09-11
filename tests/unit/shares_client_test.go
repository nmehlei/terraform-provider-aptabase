package unit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListShares_ReturnsShares(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/apps/app1/shares" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]string{{"email": "friend@example.com", "createdAt": "2026-01-01T00:00:00Z"}})
	})

	shares, err := client.ListShares(context.Background(), "app1")
	if err != nil {
		t.Fatalf("ListShares: %v", err)
	}
	if len(shares) != 1 || shares[0].Email != "friend@example.com" {
		t.Errorf("unexpected shares: %+v", shares)
	}
}

func TestAddShare_PutsToEmailPath(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v0/apps/app1/shares/friend@example.com" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{}`))
	})

	if err := client.AddShare(context.Background(), "app1", "friend@example.com"); err != nil {
		t.Fatalf("AddShare: %v", err)
	}
}

func TestRemoveShare_DeletesEmailPath(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v0/apps/app1/shares/friend@example.com" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{}`))
	})

	if err := client.RemoveShare(context.Background(), "app1", "friend@example.com"); err != nil {
		t.Fatalf("RemoveShare: %v", err)
	}
}
