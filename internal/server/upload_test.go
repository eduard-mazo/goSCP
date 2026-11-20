package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multipartBody builds an upload body. fields are written first, in order,
// followed by one "files" part per name→content pair in files.
func multipartBody(t *testing.T, fields [][2]string, files [][2]string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		w, err := mw.CreateFormFile("files", f[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, f[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func upload(h http.Handler, target string, body io.Reader, ctype string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", ctype)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertFile(t *testing.T, p, want string) {
	t.Helper()
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", p, got, want)
	}
}

// assertNoPartFiles fails if an upload left a temporary file behind.
func assertNoPartFiles(t *testing.T, dir string) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(dir, ".goscp-upload-*"))
	if len(matches) > 0 {
		t.Fatalf("leftover temp files: %v", matches)
	}
}

func TestUploadDirFromQuery(t *testing.T) {
	h, root := newTestHandler(t)
	body, ct := multipartBody(t, nil, [][2]string{{"a.txt", "alpha"}, {"b.txt", "beta"}})

	rec := upload(h, "/api/v1/upload?path=/dir", body, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	assertFile(t, filepath.Join(root, "dir", "a.txt"), "alpha")
	assertFile(t, filepath.Join(root, "dir", "b.txt"), "beta")
	assertNoPartFiles(t, filepath.Join(root, "dir"))
}

// The legacy form-field style (curl -F path=/dir -F files=@x) must keep working.
func TestUploadDirFromFormField(t *testing.T) {
	h, root := newTestHandler(t)
	body, ct := multipartBody(t, [][2]string{{"path", "/dir"}}, [][2]string{{"c.txt", "gamma"}})

	rec := upload(h, "/api/v1/upload", body, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	assertFile(t, filepath.Join(root, "dir", "c.txt"), "gamma")
}

func TestUploadOverwritesExisting(t *testing.T) {
	h, root := newTestHandler(t)
	body, ct := multipartBody(t, nil, [][2]string{{"big file.bin", "new"}})

	rec := upload(h, "/api/v1/upload?path=/", body, ct)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	assertFile(t, filepath.Join(root, "big file.bin"), "new")
}

// An upload cut off mid-stream must not truncate an existing file of the same
// name, nor leave its temp file behind.
func TestUploadAbortedKeepsExistingFile(t *testing.T) {
	h, root := newTestHandler(t)
	body, ct := multipartBody(t, nil, [][2]string{{"big file.bin", strings.Repeat("z", 4096)}})
	full, _ := io.ReadAll(body)
	truncated := bytes.NewReader(full[:len(full)/2]) // no closing boundary

	rec := upload(h, "/api/v1/upload?path=/", truncated, ct)
	if rec.Code < 400 {
		t.Fatalf("truncated upload: status %d, want an error", rec.Code)
	}
	assertFile(t, filepath.Join(root, "big file.bin"), "0123456789")
	assertNoPartFiles(t, root)
}

func TestUploadTooLarge(t *testing.T) {
	h, root := newTestHandler(t) // MaxUploadBytes = 1 MiB
	body, ct := multipartBody(t, nil, [][2]string{{"huge.bin", strings.Repeat("x", 2<<20)}})

	rec := upload(h, "/api/v1/upload?path=/", body, ct)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413: %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(root, "huge.bin")); !os.IsNotExist(err) {
		t.Fatalf("oversized upload was saved (err=%v)", err)
	}
	assertNoPartFiles(t, root)
}

func TestUploadRejectsEscapes(t *testing.T) {
	h, _ := newTestHandler(t)
	for _, name := range []string{"..", "."} {
		body, ct := multipartBody(t, nil, [][2]string{{name, "x"}})
		if rec := upload(h, "/api/v1/upload?path=/dir", body, ct); rec.Code < 400 {
			t.Fatalf("name %q: status %d, want an error", name, rec.Code)
		}
	}
}
