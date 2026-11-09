package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Signed download links
//
// Downloading through fetch() forces the browser to buffer the whole file in
// memory before it can hand it to the user, so on large files the save dialog
// (or the mobile "download?" prompt) only appears after the transfer is done.
// Instead the UI asks for a short-lived, path-scoped signed URL and points the
// browser straight at it: the native download manager then streams the file to
// disk, shows the dialog as soon as headers arrive, reports progress and can
// resume via Range requests. The bearer token itself never appears in a URL.

const (
	downloadPath = "/api/v1/download"
	// downloadLinkTTL bounds how long a signed link (and any Range resume of
	// it) stays valid. A transfer already in flight is unaffected by expiry.
	downloadLinkTTL = time.Hour
)

// downloadKey derives the HMAC key from the bearer token so link signatures are
// never usable as the token and rotate whenever the token does.
func downloadKey(token string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("goscp download link v1"))
	return m.Sum(nil)
}

func signDownload(key []byte, p string, exp int64) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(p))
	m.Write([]byte{0})
	m.Write([]byte(strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// SignedDownload reports whether r is a GET/HEAD of the download endpoint that
// carries a valid, unexpired signature for the requested path. The server's
// token guard lets such requests through without a bearer token.
func (a *API) SignedDownload(r *http.Request) bool {
	if r.URL.Path != downloadPath || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	q := r.URL.Query()
	sig, p := q.Get("sig"), q.Get("path")
	if sig == "" || p == "" {
		return false
	}
	exp, err := strconv.ParseInt(q.Get("exp"), 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(signDownload(a.downloadKey, p, exp)))
}

// handleDownloadLink returns a signed URL the browser can download natively.
// It checks the target up front so a missing file or a directory is reported
// here as JSON, instead of being saved by the browser as an error "file".
func (a *API) handleDownloadLink(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "missing path")
		return
	}
	entry, err := a.Store.Stat(p)
	if err != nil {
		a.fsError(w, err)
		return
	}
	if entry.IsDir {
		writeError(w, http.StatusBadRequest, "cannot download a directory")
		return
	}

	exp := time.Now().Add(downloadLinkTTL).Unix()
	q := url.Values{}
	q.Set("path", p)
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", signDownload(a.downloadKey, p, exp))
	writeJSON(w, http.StatusOK, map[string]any{
		"url":     downloadPath + "?" + q.Encode(),
		"expires": time.Unix(exp, 0).UTC(),
	})
}

func (a *API) handleDownload(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "missing path")
		return
	}
	f, info, err := a.Store.Open(p)
	if err != nil {
		a.fsError(w, err)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(info.Name()))
	// Never let a proxy or the browser cache a signed (credential-bearing) URL.
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// contentDisposition builds an RFC 6266 attachment header. Non-ASCII names are
// emitted in the RFC 5987 filename* form so accents and emoji survive the
// native save.
func contentDisposition(name string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		return v
	}
	return "attachment"
}
