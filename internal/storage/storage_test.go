package storage

import (
	"context"
	"errors"
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

// A cancelled context (the client navigated away) must stop a recursive walk
// instead of letting it run to completion on a large tree.
func TestWalksStopOnCancel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.Usage(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Usage err = %v, want context.Canceled", err)
	}
	if _, err := s.DirSize(ctx, "/a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DirSize err = %v, want context.Canceled", err)
	}

	du, err := s.DirSize(context.Background(), "/a")
	if err != nil {
		t.Fatal(err)
	}
	if du.FileCount != 1 || du.DirCount != 1 || du.TotalSize != 1 {
		t.Fatalf("DirSize = %+v", du)
	}
}
