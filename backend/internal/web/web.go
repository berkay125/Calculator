package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// The frontend build is copied into dist before the binary is built. all:
// keeps files whose names start with _ (Vite can emit such chunks), and the
// committed dist/.gitkeep keeps the package compiling without a build.
//
//go:embed all:dist
var embedded embed.FS

// Dist returns the frontend build embedded in the binary. Without a build it
// holds only .gitkeep.
func Dist() fs.FS {
	build, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // unreachable: "dist" is a valid path
	}
	return build
}

// contentTypes maps the file extensions of the build to their Content-Type.
// Go's own table has no font types and on Windows also reads the registry,
// so the types are fixed here.
var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// Cache-Control values. Vite puts the files whose names contain a content
// hash under assets/, so they never change. Every other file, index.html
// first of all, is revalidated with its ETag, so a new build shows up at once.
const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// file is a file of the build, read into memory with its response headers.
type file struct {
	data         []byte
	contentType  string
	cacheControl string
	etag         string
}

// NewHandler serves the files of the frontend build fsys with GET and HEAD.
// "/" serves index.html, and every other path serves the file of that name.
// Directories are not listed, files and directories whose names start with a
// dot are not served, and other methods get 405.
//
// The files are read once, here. A file with an extension missing from
// contentTypes is an error, so it fails at startup instead of being served
// with a guessed type.
func NewHandler(fsys fs.FS) (http.Handler, error) {
	files := make(map[string]file)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		f, err := readFile(fsys, name)
		if err != nil {
			return err
		}
		files["/"+name] = f
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	if index, ok := files["/index.html"]; ok {
		files["/"] = index
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		f, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", f.contentType)
		h.Set("Cache-Control", f.cacheControl)
		h.Set("ETag", f.etag)
		// ServeContent answers If-None-Match with 304 and handles HEAD and
		// Range requests. Embedded files have no modification time.
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(f.data))
	})
	return mux, nil
}

func readFile(fsys fs.FS, name string) (file, error) {
	contentType, ok := contentTypes[path.Ext(name)]
	if !ok {
		return file{}, fmt.Errorf("%s: no content type for its extension", name)
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return file{}, err
	}
	cacheControl := cacheRevalidate
	if strings.HasPrefix(name, "assets/") {
		cacheControl = cacheImmutable
	}
	sum := sha256.Sum256(data)
	return file{
		data:         data,
		contentType:  contentType,
		cacheControl: cacheControl,
		etag:         `"` + hex.EncodeToString(sum[:16]) + `"`,
	}, nil
}
