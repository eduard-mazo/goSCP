package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"goscp/internal/storage"
)

// Routes registers the JSON API on a ServeMux under /api/v1.
// Go 1.22+ method-aware pattern routing keeps this dependency-free.
func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health", a.handleHealth)
	mux.HandleFunc("POST /api/v1/token", a.handleToken)
	mux.HandleFunc("GET /api/v1/usage", a.handleUsage)
	mux.HandleFunc("GET /api/v1/dirsize", a.handleDirSize)
	mux.HandleFunc("GET /api/v1/files", a.handleList)
	mux.HandleFunc("GET /api/v1/download", a.handleDownload)
	mux.HandleFunc("GET /api/v1/download/link", a.handleDownloadLink)
	mux.HandleFunc("POST /api/v1/upload", a.handleUpload)
	mux.HandleFunc("POST /api/v1/mkdir", a.handleMkdir)
	mux.HandleFunc("POST /api/v1/rename", a.handleRename)
	mux.HandleFunc("DELETE /api/v1/files", a.handleDelete)
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// tokenRequest is the body of POST /api/v1/token.
type tokenRequest struct {
	Password string `json:"password"`
}

// handleToken exchanges the login password for the API bearer token. It is the
// only /api route left unauthenticated (see server.tokenGuard) so a client can
// obtain a token without already holding one. Password login is opt-in: with no
// --password / GOSCP_PASSWORD configured the endpoint stays disabled.
func (a *API) handleToken(w http.ResponseWriter, r *http.Request) {
	if a.Password == "" {
		writeError(w, http.StatusNotFound, "password login is not enabled")
		return
	}
	var req tokenRequest
	if err := jsonDecode(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(a.Password)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": a.Token})
}

func (a *API) handleUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := a.Store.Usage(r.Context())
	if err != nil {
		if clientGone(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

func (a *API) handleDirSize(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "missing path")
		return
	}
	du, err := a.Store.DirSize(r.Context(), p)
	if err != nil {
		if clientGone(w, err) {
			return
		}
		a.fsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, du)
}

func (a *API) handleList(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		p = "/"
	}
	listing, err := a.Store.List(p)
	if err != nil {
		a.fsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

// handleUpload streams a multipart upload straight into the target directory.
//
// It deliberately avoids r.ParseMultipartForm: that spools every file larger
// than its memory budget to os.TempDir() first and only then lets us copy it
// to the destination. On a multi-GB upload that means a full extra write after
// the client already shows 100% (the UI looks frozen) and, where /tmp is
// tmpfs, holding the whole file in RAM. Reading parts one at a time with
// MultipartReader writes each file to disk as its bytes arrive.
//
// The target directory comes from the ?path= query parameter or a "path" form
// field; a form field only applies to the files that follow it, so it must
// precede them (browsers and curl -F keep fields in the order given).
func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Refuse an oversized upload up front instead of after receiving it.
	if r.ContentLength > a.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds the server limit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected a multipart/form-data upload: "+err.Error())
		return
	}

	dir := r.URL.Query().Get("path")
	saved := make([]*storage.Entry, 0, 1)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			a.uploadError(w, err)
			return
		}
		switch name := part.FormName(); {
		case name == "path" && part.FileName() == "":
			b, err := io.ReadAll(io.LimitReader(part, 4096))
			if err != nil {
				a.uploadError(w, err)
				return
			}
			dir = strings.TrimSpace(string(b))
		case name == "files" && part.FileName() != "":
			if dir == "" {
				dir = "/"
			}
			entry, err := a.Store.Save(dir, path.Base(part.FileName()), part)
			if err != nil {
				a.uploadError(w, err)
				return
			}
			saved = append(saved, entry)
		}
		part.Close()
	}

	if len(saved) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided (field name: files)")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"saved": saved})
}

// uploadError maps a failure while streaming an upload to a response.
func (a *API) uploadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds the server limit")
		return
	}
	a.fsError(w, err)
}

// statusClientClosed is the de facto (nginx) code for "client closed the
// request". Nobody receives it; it only makes abandoned walks legible in the
// access log instead of being logged as a 200.
const statusClientClosed = 499

// clientGone reports whether err is the request context being cancelled (the
// browser aborted or navigated away) and, if so, records that status.
func clientGone(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	w.WriteHeader(statusClientClosed)
	return true
}

type nameRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (a *API) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Path == "" {
		req.Path = "/"
	}
	entry, err := a.Store.Mkdir(req.Path, req.Name)
	if err != nil {
		a.fsError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (a *API) handleRename(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	entry, err := a.Store.Rename(req.Path, req.Name)
	if err != nil {
		a.fsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" || p == "/" {
		writeError(w, http.StatusBadRequest, "missing or invalid path")
		return
	}
	if err := a.Store.Remove(p); err != nil {
		a.fsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// fsError maps storage/filesystem errors to HTTP status codes.
func (a *API) fsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrOutsideRoot):
		writeError(w, http.StatusForbidden, "path is outside the allowed root")
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, os.ErrPermission):
		writeError(w, http.StatusForbidden, "permission denied")
	case errors.Is(err, os.ErrExist):
		writeError(w, http.StatusConflict, "already exists")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer io.Copy(io.Discard, r.Body)
	if err := jsonDecode(r.Body, v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
