package grab

import (
	"context"
	"fmt"
	"hash"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// setLastModified sets the last modified timestamp of a local file according to
// the Last-Modified header returned by a remote server.
func setLastModified(resp *http.Response, filename string) error {
	// https://tools.ietf.org/html/rfc7232#section-2.2
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Last-Modified
	header := resp.Header.Get("Last-Modified")
	if header == "" {
		return nil
	}
	lastmod, err := time.Parse(http.TimeFormat, header)
	if err != nil {
		return nil
	}
	return os.Chtimes(filename, lastmod, lastmod)
}

// mkdirp creates all missing parent directories for the destination file path.
func mkdirp(path string) error {
	dir := filepath.Dir(path)
	if fi, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("error checking destination directory: %v", err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating destination directory: %v", err)
		}
	} else if !fi.IsDir() {
		panic("destination path is not directory")
	}
	return nil
}

// guessFilename returns a filename for the given http.Response.
func guessFilename(resp *http.Response) (string, error) {
	filename := resp.Request.URL.Path
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil && params["filename"] != "" {
			if hFilename, err := normalizeFilename(params["filename"]); err == nil {
				return hFilename, nil
			}
		}
	}

	return normalizeFilename(filename)
}

// normalizeFilename sanitizes and strips filename from unnecessary symbols.
// If none can be determined ErrNoFilename is returned.
func normalizeFilename(filename string) (string, error) {
	// sanitize
	if filename == "" || strings.HasSuffix(filename, "/") || strings.Contains(filename, "\x00") {
		return "", ErrNoFilename
	}

	filename = filepath.Base(path.Clean("/" + filename))
	if filename == "" || filename == "." || filename == "/" {
		return "", ErrNoFilename
	}

	return filename, nil
}

// checksum returns a hash of the given file, using the given hash algorithm.
func checksum(ctx context.Context, filename string, h hash.Hash) (b []byte, err error) {
	var f *os.File
	f, err = os.Open(filename)
	if err != nil {
		return
	}
	defer func() {
		err = f.Close()
	}()

	t := newTransfer(ctx, nil, h, f, nil)
	if _, err = t.copy(); err != nil {
		return
	}

	b = h.Sum(nil)
	return
}
