package acceptance

// Bootstrap signs up a fresh aptabase-plus user via the real magic-link
// email flow (reading the email from the disposable Mailcatcher instance
// started by up.sh) and mints an API key for that user, since Terraform
// itself has no way to complete an interactive sign-in. Every acceptance
// test in this package calls this once and configures its provider block
// with the returned endpoint and apiKey.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	// Ports match the host-side mappings in docker-compose.yml, which
	// are remapped off the aptabase-plus defaults (8080/1080) to avoid
	// colliding with the separate aptabase-plus dev sandbox that may
	// already be running locally.
	aptabasePlusURL = "http://localhost:18080"
	mailcatcherURL  = "http://localhost:11080"
)

var continueLinkPattern = regexp.MustCompile(`href="([^"]*api/_auth/continue[^"]*)"`)

// Bootstrap returns (endpoint, apiKey) for a freshly created aptabase-plus
// account with one API key minted.
func Bootstrap(t *testing.T) (string, string) {
	t.Helper()

	email := fmt.Sprintf("acc-test-%d@example.com", time.Now().UnixNano())
	client := &http.Client{Timeout: 10 * time.Second}

	registerBody := fmt.Sprintf(`{"name":"Acceptance Test","email":%q}`, email)
	resp, err := client.Post(aptabasePlusURL+"/api/_auth/register", "application/json", strings.NewReader(registerBody))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register: unexpected status %d", resp.StatusCode)
	}

	continueURL := waitForConfirmationLink(t, client, email)

	jar := newCookieClient(t)
	resp, err = jar.Get(continueURL)
	if err != nil {
		t.Fatalf("following confirmation link: %v", err)
	}
	resp.Body.Close()

	createKeyBody := `{"name":"acceptance-test"}`
	resp, err = jar.Post(aptabasePlusURL+"/api/v0/api-keys", "application/json", strings.NewReader(createKeyBody))
	if err != nil {
		t.Fatalf("creating api key: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("creating api key: status %d: %s", resp.StatusCode, body)
	}

	var created struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decoding api key response: %v", err)
	}

	return aptabasePlusURL, created.Key
}

// newCookieClient returns an http.Client that persists cookies across
// requests, needed to hold the auth-session cookie set by following the
// confirmation link.
func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("creating cookie jar: %v", err)
	}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

// waitForConfirmationLink polls Mailcatcher for the registration email
// sent to `email` and extracts the /api/_auth/continue link from its HTML
// body. Verified against aptabase-plus's Register.html email template
// (src/assets/Templates/Register.html), which renders a plain
// `href="##URL##"` anchor with no query-string escaping, and against the
// dockage/mailcatcher:0.8.2 image pinned in docker-compose.yml, whose
// /messages and /messages/:id.html REST endpoints implement the standard
// Mailcatcher API.
func waitForConfirmationLink(t *testing.T, client *http.Client, email string) string {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		messages, err := fetchMailcatcherMessages(client)
		if err == nil {
			for i := len(messages) - 1; i >= 0; i-- {
				if !messageIsTo(messages[i], email) {
					continue
				}
				html, err := fetchMailcatcherMessageHTML(client, messages[i].ID)
				if err != nil {
					continue
				}
				if m := continueLinkPattern.FindStringSubmatch(html); m != nil {
					return htmlUnescapeAmp(m[1])
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no confirmation email arrived for %s within 15s", email)
	return ""
}

type mailcatcherMessage struct {
	ID         int      `json:"id"`
	Recipients []string `json:"recipients"`
}

func fetchMailcatcherMessages(client *http.Client) ([]mailcatcherMessage, error) {
	resp, err := client.Get(mailcatcherURL + "/messages")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var messages []mailcatcherMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func fetchMailcatcherMessageHTML(client *http.Client, id int) (string, error) {
	resp, err := client.Get(fmt.Sprintf("%s/messages/%d.html", mailcatcherURL, id))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func messageIsTo(m mailcatcherMessage, email string) bool {
	for _, r := range m.Recipients {
		if strings.Contains(r, email) {
			return true
		}
	}
	return false
}

func htmlUnescapeAmp(s string) string {
	return strings.ReplaceAll(s, "&amp;", "&")
}
