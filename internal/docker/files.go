package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// CopyFileToContainer writes data to dstPath inside the container, creating
// or truncating the file. The mode is applied to the file inside the tar.
func CopyFileToContainer(ctx context.Context, cli client.APIClient, containerID, dstPath string, data []byte, mode fs.FileMode) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: path.Base(dstPath),
		Mode: int64(mode.Perm()),
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write tar body: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}

	dstDir := path.Dir(dstPath)
	if dstDir == "" {
		dstDir = "/"
	}

	if err := cli.CopyToContainer(ctx, containerID, dstDir, &buf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}
	return nil
}

// CopyFileFromContainer reads the file at srcPath inside the container and
// returns its contents. The path must point to a regular file.
func CopyFileFromContainer(ctx context.Context, cli client.APIClient, containerID, srcPath string) ([]byte, error) {
	rc, _, err := cli.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, &fs.PathError{Op: "open", Path: srcPath, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	if err == io.EOF {
		return nil, fmt.Errorf("empty tar stream reading %s", srcPath)
	}
	if err != nil {
		return nil, fmt.Errorf("read tar header: %w", err)
	}
	if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
		return nil, fmt.Errorf("expected regular file at %s, got tar typeflag %c", srcPath, hdr.Typeflag)
	}
	return io.ReadAll(tr)
}

// StatFileInContainer returns size, mode, and is-directory info for path.
// If the path does not exist the returned error wraps fs.ErrNotExist.
func StatFileInContainer(ctx context.Context, cli client.APIClient, containerID, p string) (size int64, mode fs.FileMode, isDir bool, err error) {
	stat, err := cli.ContainerStatPath(ctx, containerID, p)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return 0, 0, false, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
		}
		return 0, 0, false, err
	}
	return stat.Size, stat.Mode, stat.Mode.IsDir(), nil
}

// RemoveFileInContainer deletes a file via `rm -f`. Returns an error wrapping
// fs.ErrNotExist if the path was missing.
func RemoveFileInContainer(cli client.APIClient, containerID, p string) error {
	// First check existence so we can return a typed not-exist error.
	if _, _, _, err := StatFileInContainer(context.Background(), cli, containerID, p); err != nil {
		return err
	}
	code, _, stderr, err := RunCommandInContainer(cli, containerID, fmt.Sprintf("rm -f -- %s", shellQuote(p)))
	if err != nil {
		return err
	}
	if code != 0 {
		errOut, _ := io.ReadAll(stderr)
		return fmt.Errorf("rm exited %d: %s", code, bytes.TrimSpace(errOut))
	}
	return nil
}

// MkdirAllInContainer creates the directory at p (and any missing parents)
// using `mkdir -p`, then chmods the leaf to the requested mode.
func MkdirAllInContainer(cli client.APIClient, containerID, p string, mode fs.FileMode) error {
	cmd := fmt.Sprintf("mkdir -p -- %s && chmod %o -- %s", shellQuote(p), mode.Perm(), shellQuote(p))
	code, _, stderr, err := RunCommandInContainer(cli, containerID, cmd)
	if err != nil {
		return err
	}
	if code != 0 {
		errOut, _ := io.ReadAll(stderr)
		return errors.New(string(bytes.TrimSpace(errOut)))
	}
	return nil
}

// shellQuote wraps s in single quotes and escapes any embedded single quotes,
// producing a value safe to splice into a sh -c command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
