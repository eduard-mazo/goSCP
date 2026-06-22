package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestListFollowsSymlinkedDir reproduces the Termux/Android layout where the
// served root (~/storage) is a directory of symlinks into shared storage. A
// symlink pointing at a directory must be listed as a directory so the UI can
// navigate into it.
func TestListFollowsSymlinkedDir(t *testing.T) {
	root := t.TempDir()

	// A real directory outside the root, plus a file inside it.
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// downloads -> target, mimicking ~/storage/downloads.
	link := filepath.Join(root, "downloads")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	s := New(root)

	listing, err := s.List("/")
	if err != nil {
		t.Fatalf("List(/): %v", err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(listing.Entries))
	}
	if e := listing.Entries[0]; !e.IsDir {
		t.Fatalf("symlinked dir %q reported IsDir=false (Mode=%s)", e.Name, e.Mode)
	}

	// And navigating into it must reveal the target's contents.
	sub, err := s.List("/downloads")
	if err != nil {
		t.Fatalf("List(/downloads): %v", err)
	}
	if len(sub.Entries) != 1 || sub.Entries[0].Name != "inside.txt" {
		t.Fatalf("unexpected contents of linked dir: %+v", sub.Entries)
	}
}

// TestListDanglingSymlink ensures a broken link does not break listing.
func TestListDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "ghost")
	if err := os.Symlink(filepath.Join(root, "nope"), link); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	s := New(root)
	listing, err := s.List("/")
	if err != nil {
		t.Fatalf("List(/): %v", err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(listing.Entries))
	}
	if listing.Entries[0].IsDir {
		t.Fatalf("dangling symlink should not be reported as a directory")
	}
}
