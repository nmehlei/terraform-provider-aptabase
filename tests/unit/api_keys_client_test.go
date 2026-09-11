package unit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateApiKey_ReturnsPlaintextKeyOnce(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v0/api-keys/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"id": "key1", "name": "ci", "keyPrefix": "aptb_AbCd12", "createdAt": "2026-01-01T00:00:00Z",
			"key": "aptb_AbCd1234567890",
		})
	})

	created, err := client.CreateApiKey(context.Background(), "ci", nil)
	if err != nil {
		t.Fatalf("CreateApiKey: %v", err)
	}
	if created.Key != "aptb_AbCd1234567890" {
		t.Errorf("unexpected created key: %+v", created)
	}
}

func TestListApiKeys_ReturnsSummariesWithoutPlaintext(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{
			{"id": "key1", "name": "ci", "keyPrefix": "aptb_AbCd12", "createdAt": "2026-01-01T00:00:00Z"},
		})
	})

	keys, err := client.ListApiKeys(context.Background())
	if err != nil {
		t.Fatalf("ListApiKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != "key1" {
		t.Errorf("unexpected keys: %+v", keys)
	}
}

func TestDeleteApiKey_Succeeds(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v0/api-keys/key1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{}`))
	})

	if err := client.DeleteApiKey(context.Background(), "key1"); err != nil {
		t.Fatalf("DeleteApiKey: %v", err)
	}
}
