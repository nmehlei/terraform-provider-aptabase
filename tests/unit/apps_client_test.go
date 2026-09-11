package unit_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nmehlei/terraform-provider-aptabase/src/aptabase"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *aptabase.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := aptabase.New(aptabase.Config{
		Endpoint:      server.URL,
		Token:         "aptb_test",
		AllowInsecure: true,
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("aptabase.New: %v", err)
	}
	return client
}

func TestCreateApp_SendsNameAndDecodesResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v0/apps/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer aptb_test" {
			t.Errorf("Authorization header = %q", got)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "My App" {
			t.Errorf("request body name = %q", body["name"])
		}
		json.NewEncoder(w).Encode(aptabase.App{ID: "app1", Name: "My App", AppKey: "A-US-1234567890"})
	})

	app, err := client.CreateApp(context.Background(), "My App")
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if app.ID != "app1" || app.AppKey != "A-US-1234567890" {
		t.Errorf("unexpected app: %+v", app)
	}
}

func TestGetApp_NotFound_ReturnsAPIErrorWithNotFoundTrue(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{}`))
	})

	_, err := client.GetApp(context.Background(), "missing")
	var apiErr *aptabase.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *aptabase.APIError, got %T: %v", err, err)
	}
	if !apiErr.NotFound() {
		t.Errorf("expected NotFound() true for a 404")
	}
}

func TestUpdateApp_SendsNameAndIcon(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v0/apps/app1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "Renamed" || body["icon"] != "aWNvbg==" {
			t.Errorf("unexpected request body: %+v", body)
		}
		json.NewEncoder(w).Encode(aptabase.App{ID: "app1", Name: "Renamed"})
	})

	app, err := client.UpdateApp(context.Background(), "app1", "Renamed", "aWNvbg==")
	if err != nil {
		t.Fatalf("UpdateApp: %v", err)
	}
	if app.Name != "Renamed" {
		t.Errorf("unexpected app: %+v", app)
	}
}

func TestDeleteApp_Succeeds(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v0/apps/app1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{}`))
	})

	if err := client.DeleteApp(context.Background(), "app1"); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
}
