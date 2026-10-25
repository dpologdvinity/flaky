package flaky

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func workspaceTemp(src, prefix string) (string, error) {
	source, err := filepath.EvalSymlinks(src)
	if err != nil {
		return "", err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(source, parent)
	if err == nil && (rel == "." || filepath.IsLocal(rel)) {
		return "", fmt.Errorf("temporary directory must be outside source: %s", parent)
	}
	return os.MkdirTemp(parent, prefix)
}

func writableWorkspace(path string) error {
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return os.Chmod(path, 0700)
		}
		if d.Type().IsRegular() {
			return os.Chmod(path, 0600)
		}
		return nil
	})
}

func removeWorkspace(path string) error {
	return errors.Join(writableWorkspace(path), os.RemoveAll(path))
}

type copyReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r copyReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func CopyWorkspace(src string) (string, func() error, error) {
	return copyWorkspace(context.Background(), src)
}

func copyWorkspace(ctx context.Context, src string) (string, func() error, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	dst, err := workspaceTemp(src, "flaky-run-")
	if err != nil {
		return "", nil, err
	}
	clean := func() error { return removeWorkspace(dst) }
	type directory struct {
		path string
		mode fs.FileMode
	}
	var directories []directory
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			directories = append(directories, directory{target, info.Mode().Perm()})
			return os.MkdirAll(target, info.Mode().Perm()|0700)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported workspace file: %s", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, copyReader{ctx, in})
		closeErr := out.Close()
		readCloseErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return readCloseErr
	})
	if err == nil {
		for i := len(directories) - 1; i >= 0; i-- {
			if err = ctx.Err(); err != nil {
				break
			}
			if err = os.Chmod(directories[i].path, directories[i].mode); err != nil {
				break
			}
		}
	}
	if err != nil {
		return "", nil, errors.Join(err, clean())
	}
	return dst, clean, nil
}
