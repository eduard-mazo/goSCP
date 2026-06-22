// Package storage provides path-confined filesystem operations rooted at a
// single base directory. Every public method validates that the requested
// path resolves inside the root, preventing path-traversal escapes.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrOutsideRoot is returned when a requested path escapes the storage root.
var ErrOutsideRoot = errors.New("path escapes root")

// Store performs file operations confined to Root.
type Store struct {
	Root string
}

// New returns a Store rooted at an absolute directory.
func New(root string) *Store {
	return &Store{Root: root}
}

// Entry describes a single file or directory.
type Entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"` // POSIX-style path relative to root, leading "/"
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Mode    string    `json:"mode"`
}

// Listing is the response for a directory read.
type Listing struct {
	Path    string  `json:"path"`
	Parent  string  `json:"parent"`
	Entries []Entry `json:"entries"`
}

// resolve cleans a client-supplied path and maps it to an absolute on-disk
// path, guaranteeing the result stays within Root.
func (s *Store) resolve(rel string) (string, error) {
	// Normalise to a rooted, cleaned POSIX path then strip the leading slash.
	clean := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))
	abs := filepath.Join(s.Root, filepath.FromSlash(clean))

	// Defence in depth: ensure the joined path is still under Root.
	relCheck, err := filepath.Rel(s.Root, abs)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(os.PathSeparator)) {
		return "", ErrOutsideRoot
	}
	return abs, nil
}

// toRel converts an absolute on-disk path back to a POSIX path relative to root.
func (s *Store) toRel(abs string) string {
	r, err := filepath.Rel(s.Root, abs)
	if err != nil {
		return "/"
	}
	r = filepath.ToSlash(r)
	if r == "." {
		return "/"
	}
	return "/" + r
}

// List returns the directory contents at rel, sorted dirs-first then by name.
func (s *Store) List(rel string) (*Listing, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	dirents, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(dirents))
	for _, de := range dirents {
		info, err := de.Info()
		if err != nil {
			continue // skip entries that vanished mid-listing
		}

		isDir, size, modTime, mode := de.IsDir(), info.Size(), info.ModTime(), info.Mode()

		// os.ReadDir reports symlinks with Lstat semantics, so a linked
		// directory would surface as a plain file (IsDir=false) and become
		// un-navigable. This is the norm on Termux/Android, where ~/storage is
		// a directory of symlinks into shared storage (e.g. downloads ->
		// /storage/emulated/0/Download). Follow the link with Stat so the entry
		// reflects its real target; if the link dangles, keep the Lstat data.
		if mode&os.ModeSymlink != 0 {
			if target, terr := os.Stat(filepath.Join(abs, de.Name())); terr == nil {
				isDir, size, modTime, mode = target.IsDir(), target.Size(), target.ModTime(), target.Mode()
			}
		}

		entries = append(entries, Entry{
			Name:    de.Name(),
			Path:    path.Join(s.toRel(abs), de.Name()),
			IsDir:   isDir,
			Size:    size,
			ModTime: modTime,
			Mode:    mode.String(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	cur := s.toRel(abs)
	return &Listing{
		Path:    cur,
		Parent:  path.Dir(cur),
		Entries: entries,
	}, nil
}

// Stat returns metadata for a single path.
func (s *Store) Stat(rel string) (*Entry, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	return &Entry{
		Name:    info.Name(),
		Path:    s.toRel(abs),
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Mode:    info.Mode().String(),
	}, nil
}

// Open opens a file for reading along with its metadata. The caller must Close.
func (s *Store) Open(rel string) (*os.File, os.FileInfo, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, fmt.Errorf("%q is a directory", rel)
	}
	return f, info, nil
}

// Save writes r to a new file named name inside the directory dir.
func (s *Store) Save(dir, name string, r io.Reader) (*Entry, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid file name %q", name)
	}
	abs, err := s.resolve(path.Join(dir, name))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return nil, err
	}
	return s.Stat(s.toRel(abs))
}

// Mkdir creates a directory named name inside dir.
func (s *Store) Mkdir(dir, name string) (*Entry, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid directory name %q", name)
	}
	abs, err := s.resolve(path.Join(dir, name))
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(abs, 0o755); err != nil {
		return nil, err
	}
	return s.Stat(s.toRel(abs))
}

// Remove deletes a file or directory (recursively) at rel.
func (s *Store) Remove(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if abs == s.Root {
		return errors.New("cannot remove root")
	}
	return os.RemoveAll(abs)
}

// Rename moves/renames a path to a new name within the same directory.
func (s *Store) Rename(rel, newName string) (*Entry, error) {
	if newName == "" || strings.ContainsAny(newName, `/\`) {
		return nil, fmt.Errorf("invalid name %q", newName)
	}
	oldAbs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	newAbs, err := s.resolve(path.Join(path.Dir(rel), newName))
	if err != nil {
		return nil, err
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return nil, err
	}
	return s.Stat(s.toRel(newAbs))
}

// DiskUsage reports capacity for the volume backing the root directory.
type DiskUsage struct {
	Root      string `json:"root"`
	TotalSize int64  `json:"totalSize"` // sum of file sizes under root
	FileCount int    `json:"fileCount"`
	DirCount  int    `json:"dirCount"`
}

// Usage walks the root and tallies basic statistics.
func (s *Store) Usage() (*DiskUsage, error) {
	du := &DiskUsage{Root: s.Root}
	err := filepath.WalkDir(s.Root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore unreadable entries
		}
		if d.IsDir() {
			du.DirCount++
			return nil
		}
		du.FileCount++
		if info, e := d.Info(); e == nil {
			du.TotalSize += info.Size()
		}
		return nil
	})
	return du, err
}
