// Package external_api contains external black-box HTTP tests that cover
// every API endpoint not already exercised by tests/blackbox/.
//
// Every request in this package goes through a real TCP socket via
// http.Client. No ServeHTTP(), no httptest.NewRecorder(), no HTTP-layer
// mocks. The target server is either:
//
//   - an already-running app accessed via EXTERNAL_API_BASE_URL (e.g.
//     the Docker Compose app service — "CI-like" mode), OR
//   - a local httptest.NewServer wrapping the real router + real DB
//     (when EXTERNAL_API_BASE_URL is unset — "fast" mode).
//
// Both modes satisfy the requirement: external HTTP, real app code,
// real persistence. The Docker-hosted mode is the one documented in
// README for CI.
package external_api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/campusrec/campusrec/tests/testutil"
)

// envelope mirrors the standard API response envelope.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta *struct {
		RequestID string `json:"request_id"`
		Total     int    `json:"total"`
	} `json:"meta"`
}

type testEnv struct {
	BaseURL string
}

// setupEnv returns an external test environment. Uses EXTERNAL_API_BASE_URL
// if set (CI / Docker Compose mode), otherwise boots a local httptest.Server.
func setupEnv(t *testing.T) *testEnv {
	t.Helper()

	if base := os.Getenv("EXTERNAL_API_BASE_URL"); base != "" {
		// Against an already-running Docker-hosted app.
		return &testEnv{BaseURL: base}
	}

	// Fall back to an in-test real HTTP server. Requires DATABASE_URL.
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("neither EXTERNAL_API_BASE_URL nor DATABASE_URL is set")
	}
	r, _ := testutil.SetupTestRouter(t)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &testEnv{BaseURL: srv.URL}
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// call issues an HTTP request and decodes the envelope.
func call(c *http.Client, method, url, body string) (*http.Response, envelope) {
	var r io.Reader
	if body != "" {
		r = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, url, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, envelope{}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp, env
}

// rawCall returns the full body without unmarshalling (for non-JSON endpoints).
func rawCall(c *http.Client, method, url string) (*http.Response, []byte) {
	req, _ := http.NewRequest(method, url, nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func loginAs(t *testing.T, base, user, pass string) *http.Client {
	t.Helper()
	c := newClient()
	resp, env := call(c, "POST", base+"/api/v1/auth/login",
		fmt.Sprintf(`{"username":%q,"password":%q}`, user, pass))
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("login %s failed: %+v", user, env)
	}
	return c
}

func registerAndLogin(t *testing.T, base, user string) (*http.Client, string) {
	t.Helper()
	c := newClient()
	resp, env := call(c, "POST", base+"/api/v1/auth/register",
		fmt.Sprintf(`{"username":%q,"password":"SecurePass123!","display_name":"Test %s"}`, user, user))
	if resp == nil || resp.StatusCode != 201 {
		t.Fatalf("register %s: %d", user, resp.StatusCode)
	}
	newID := ds(env, "id")
	return loginAs(t, base, user, "SecurePass123!"), newID
}

func dmap(env envelope) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal(env.Data, &m)
	return m
}

func dlist(env envelope) []map[string]interface{} {
	var l []map[string]interface{}
	_ = json.Unmarshal(env.Data, &l)
	return l
}

func ds(env envelope, k string) string {
	m := dmap(env)
	if m == nil {
		return ""
	}
	v, _ := m[k].(string)
	return v
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of message keyed by key.
// The payment service expects this exact format for callback signatures.
func computeHMAC(message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// uploadFile posts a multipart form to an import endpoint.
func uploadFile(t *testing.T, c *http.Client, url, filename, content, templateType string) (*http.Response, envelope) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filename)
	_, _ = fw.Write([]byte(content))
	_ = w.WriteField("template_type", templateType)
	_ = w.Close()

	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp, env
}
