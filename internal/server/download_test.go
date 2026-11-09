package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goscp/internal/config"
)

const testToken = "test-token"

func newTestHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big file.bin"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "café.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(&config.Config{Root: root, Token: testToken, MaxUploadBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return s.http.Handler, root
}

func do(h http.Handler, method, target string, bearer bool, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if bearer {
		req.Header.Set("Authorization", "Bearer "+testToken)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func signedLink(t *testing.T, h http.Handler, p string) string {
	t.Helper()
	rec := do(h, http.MethodGet, "/api/v1/download/link?path="+url.QueryEscape(p), true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("link for %q: status %d: %s", p, rec.Code, rec.Body)
	}
	var body struct{ URL string }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body.URL, testToken) {
		t.Fatalf("signed URL leaks the bearer token: %s", body.URL)
	}
	return body.URL
}

func TestSignedDownloadStreamsWithoutBearer(t *testing.T) {
	h, _ := newTestHandler(t)
	link := signedLink(t, h, "/big file.bin")

	rec := do(h, http.MethodGet, link, false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed download: status %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); got != "0123456789" {
		t.Fatalf("body = %q", got)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="big file.bin"` {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if rec.Header().Get("Content-Length") != "10" {
		t.Fatalf("Content-Length = %q", rec.Header().Get("Content-Length"))
	}

	// Range requests must keep working so the browser can resume.
	rec = do(h, http.MethodGet, link, false, map[string]string{"Range": "bytes=5-"})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "56789" {
		t.Fatalf("range: status %d body %q", rec.Code, rec.Body)
	}
}

func TestSignedDownloadRejectsTampering(t *testing.T) {
	h, _ := newTestHandler(t)
	link := signedLink(t, h, "/big file.bin")
	u, _ := url.Parse(link)

	cases := map[string]func(q url.Values){
		"other path":  func(q url.Values) { q.Set("path", "/café.txt") },
		"bad sig":     func(q url.Values) { q.Set("sig", "AAAA") },
		"no sig":      func(q url.Values) { q.Del("sig") },
		"pushed exp":  func(q url.Values) { q.Set("exp", "99999999999") },
		"expired exp": func(q url.Values) { q.Set("exp", "1") },
	}
	for name, mutate := range cases {
		q := u.Query()
		mutate(q)
		rec := do(h, http.MethodGet, u.Path+"?"+q.Encode(), false, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, rec.Code)
		}
	}

	// A signature must not unlock any endpoint other than the download.
	rec := do(h, http.MethodGet, "/api/v1/files?"+u.RawQuery, false, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("signature reused on /files: status %d, want 401", rec.Code)
	}
}

func TestDownloadLinkValidatesTarget(t *testing.T) {
	h, _ := newTestHandler(t)

	if rec := do(h, http.MethodGet, "/api/v1/download/link?path=/dir", true, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("directory: status %d, want 400", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/v1/download/link?path=/missing", true, nil); rec.Code != http.StatusNotFound {
		t.Errorf("missing: status %d, want 404", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/v1/download/link?path=/big%20file.bin", false, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("link without bearer: status %d, want 401", rec.Code)
	}
}

func TestDownloadUnicodeFilename(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := do(h, http.MethodGet, signedLink(t, h, "/café.txt"), false, nil)
	if cd := rec.Header().Get("Content-Disposition"); cd != "attachment; filename*=utf-8''caf%C3%A9.txt" {
		t.Fatalf("Content-Disposition = %q", cd)
	}
}

func TestBearerDownloadStillWorks(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := do(h, http.MethodGet, "/api/v1/download?path=/big%20file.bin", true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	if string(b) != "0123456789" {
		t.Fatalf("body = %q", b)
	}
}
