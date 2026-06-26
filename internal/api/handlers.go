package api

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"

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

func (a *API) handleUsage(w http.ResponseWriter, _ *http.Request) {
	usage, err := a.Store.Usage()
	if err != nil {
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
	du, err := a.Store.DirSize(p)
	if err != nil {
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
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(info.Name()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed: "+err.Error())
		return
	}
	dir := r.FormValue("path")
	if dir == "" {
		dir = "/"
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided (field name: files)")
		return
	}

	saved := make([]*storage.Entry, 0, len(files))
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		entry, err := a.Store.Save(dir, path.Base(fh.Filename), src)
		src.Close()
		if err != nil {
			a.fsError(w, err)
			return
		}
		saved = append(saved, entry)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"saved": saved})
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
