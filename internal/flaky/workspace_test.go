package flaky

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyWorkspace(t *testing.T) {
	src := t.TempDir()
	if err := os.Mkdir(filepath.Join(src, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".git", "secret"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	dst, clean, err := CopyWorkspace(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatal("git copied")
	}
	if b, err := os.ReadFile(filepath.Join(dst, "a")); err != nil || string(b) != "ok" {
		t.Fatal(err)
	}
	if x, err := os.Readlink(filepath.Join(dst, "link")); err != nil || x != "a" {
		t.Fatal(x, err)
	}
	clean()
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("cleanup")
	}
}
