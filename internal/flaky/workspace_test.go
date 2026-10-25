package flaky

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestCopyRejectsTemporaryDirectoryInsideSource(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "symlink"}[alias], func(t *testing.T) {
			src, outside := t.TempDir(), t.TempDir()
			parent := filepath.Join(src, "tmp")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			if alias {
				link := filepath.Join(outside, "alias")
				if err := os.Symlink(parent, link); err != nil {
					t.Fatal(err)
				}
				parent = link
			}
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, parent)
			}
			_, clean, err := CopyWorkspace(src)
			if clean != nil {
				clean()
			}
			if err == nil || !strings.Contains(err.Error(), "temporary directory") {
				t.Fatalf("want unsafe temporary directory error, got %v", err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatalf("source was modified: %v, %v", entries, err)
			}
		})
	}
}

func TestCopyReadOnlyDirectoriesAndCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	src := t.TempDir()
	fixture := filepath.Join(src, "fixture")
	if err := os.Mkdir(fixture, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "data"), []byte("fixture"), 0444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fixture, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(fixture, 0700) })
	dst, clean, err := CopyWorkspace(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clean() })
	data, err := os.ReadFile(filepath.Join(dst, "fixture", "data"))
	if err != nil || string(data) != "fixture" {
		t.Fatalf("copy: %q %v", data, err)
	}
	info, err := os.Stat(filepath.Join(dst, "fixture"))
	if err != nil || info.Mode().Perm() != 0555 {
		t.Fatalf("directory permissions: %v %v", info, err)
	}
	clean()
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("workspace remains: %v", err)
	}
}

func TestCleanupRemovesReadOnlyRunArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	src, outside := t.TempDir(), t.TempDir()
	if err := os.Chmod(outside, 0755); err != nil {
		t.Fatal(err)
	}
	dst, clean, err := CopyWorkspace(src)
	if err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dst, "locked")
	t.Cleanup(func() { _ = os.Chmod(locked, 0700); clean() })
	if err := os.Mkdir(locked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "data"), []byte("x"), 0444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dst, "outside")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatal(err)
	}
	clean()
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("workspace remains: %v", err)
	}
	info, err := os.Stat(outside)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("cleanup followed symlink: %v %v", info, err)
	}
}

func TestCanceledRunDoesNotCopySource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, filepath.Join(t.TempDir(), "absent"), []string{"true"}, "report.xml")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation before copying, got %v", err)
	}
}

type cancelAfterRead struct {
	io.Reader
	cancel context.CancelFunc
}

func (r cancelAfterRead) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.cancel()
	return n, err
}

func TestFileCopyStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := cancelAfterRead{strings.NewReader(strings.Repeat("x", 65536)), cancel}
	n, err := io.Copy(io.Discard, copyReader{ctx, source})
	if !errors.Is(err, context.Canceled) || n <= 0 || n >= 65536 {
		t.Fatalf("copy continued after cancellation: %d bytes, %v", n, err)
	}
}

func TestCleanupPreservesExternalHardlinkPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions")
	}
	src, outside := t.TempDir(), t.TempDir()
	executable := filepath.Join(outside, "tool")
	if err := os.WriteFile(executable, []byte("external"), 0755); err != nil {
		t.Fatal(err)
	}
	dst, clean, err := CopyWorkspace(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clean() })
	if err := os.Link(executable, filepath.Join(dst, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := clean(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(executable)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("external hardlink permissions changed: %v %v", info, err)
	}
}

func TestCopyRejectsTemporaryDirectoryInParentRepository(t *testing.T) {
	repo := fixtureRepo(t)
	for _, name := range []string{"package", "tmp"} {
		if err := os.Mkdir(filepath.Join(repo, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	parent := filepath.Join(repo, "tmp")
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, parent)
	}
	_, clean, err := CopyWorkspace(filepath.Join(repo, "package"))
	if clean != nil {
		clean()
	}
	if err == nil || !strings.Contains(err.Error(), "temporary directory") {
		t.Fatalf("temporary writes inside parent repository accepted: %v", err)
	}
}
