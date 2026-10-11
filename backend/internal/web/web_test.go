package web

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// build is a frontend build laid out the way Vite writes it.
var build = fstest.MapFS{
	"index.html":                 {Data: []byte("<!doctype html><title>Calculator</title>")},
	"favicon.svg":                {Data: []byte("<svg></svg>")},
	"assets/index-Bn2u2.js":      {Data: []byte("console.log(1)")},
	"assets/_helper-x1.js":       {Data: []byte("export {}")},
	"assets/index-DAcP.css":      {Data: []byte("body{}")},
	"assets/plex-400-BSMl.woff":  {Data: []byte("wOFF")},
	"assets/plex-400-CEL4.woff2": {Data: []byte("wOF2")},
	".gitkeep":                   {},
	"assets/.DS_Store":           {Data: []byte("finder")},
	".vite/manifest.json":        {Data: []byte("{}")},
}

func newHandler(t *testing.T, fsys fs.FS) http.Handler {
	t.Helper()
	h, err := NewHandler(fsys)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func serve(h http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServe(t *testing.T) {
	h := newHandler(t, build)
	tests := []struct {
		path         string
		body         string
		contentType  string
		cacheControl string
	}{
		{"/", "<!doctype html><title>Calculator</title>", "text/html; charset=utf-8", "no-cache"},
		{"/index.html", "<!doctype html><title>Calculator</title>", "text/html; charset=utf-8", "no-cache"},
		{"/favicon.svg", "<svg></svg>", "image/svg+xml", "no-cache"},
		{"/assets/index-Bn2u2.js", "console.log(1)", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/_helper-x1.js", "export {}", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/index-DAcP.css", "body{}", "text/css; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/plex-400-BSMl.woff", "wOFF", "font/woff", "public, max-age=31536000, immutable"},
		{"/assets/plex-400-CEL4.woff2", "wOF2", "font/woff2", "public, max-age=31536000, immutable"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := serve(h, http.MethodGet, tt.path, nil)
			if rec.Code != http.StatusOK || rec.Body.String() != tt.body {
				t.Fatalf("GET %s = %d %q, want 200 %q", tt.path, rec.Code, rec.Body, tt.body)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tt.contentType)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
		})
	}
}

// Only the files of the build are served: no directory listings, no dotfiles
// and no fallback to index.html, since the app has no client-side routes.
func TestNotFound(t *testing.T) {
	h := newHandler(t, build)
	for _, path := range []string{
		"/missing",
		"/missing.js",
		"/assets",
		"/assets/",
		"/index.html/",
		"/.gitkeep",
		"/assets/.DS_Store",
		"/.vite/manifest.json",
	} {
		if rec := serve(h, http.MethodGet, path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d %q, want 404", path, rec.Code, rec.Body)
		}
	}
}

func TestMethods(t *testing.T) {
	h := newHandler(t, build)
	rec := serve(h, http.MethodHead, "/", nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD / = %d %q, want 200 with no body", rec.Code, rec.Body)
	}
	if got, want := rec.Header().Get("Content-Length"), "40"; got != want {
		t.Errorf("HEAD / Content-Length = %q, want %q", got, want)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := serve(h, method, "/", nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s / = %d, want 405", method, rec.Code)
		}
		if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
			t.Errorf("%s / Allow = %q, want %q", method, got, want)
		}
	}
}

func TestETag(t *testing.T) {
	h := newHandler(t, build)
	etag := serve(h, http.MethodGet, "/", nil).Header().Get("ETag")
	if len(etag) != 34 || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("ETag = %q, want 32 hex digits in quotes", etag)
	}
	if other := serve(h, http.MethodGet, "/favicon.svg", nil).Header().Get("ETag"); other == etag {
		t.Errorf("index.html and favicon.svg have the same ETag %s", etag)
	}

	// A browser revalidating index.html gets 304 while the file is unchanged.
	rec := serve(h, http.MethodGet, "/", http.Header{"If-None-Match": {etag}})
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("GET / with its ETag = %d %q, want 304 with no body", rec.Code, rec.Body)
	}
	rec = serve(h, http.MethodGet, "/", http.Header{"If-None-Match": {`"stale"`}})
	if rec.Code != http.StatusOK {
		t.Errorf("GET / with a stale ETag = %d, want 200", rec.Code)
	}
}

// Without a frontend build there is nothing to serve, but the handler works.
func TestEmptyBuild(t *testing.T) {
	h := newHandler(t, fstest.MapFS{".gitkeep": {}})
	if rec := serve(h, http.MethodGet, "/", nil); rec.Code != http.StatusNotFound {
		t.Errorf("GET / = %d, want 404", rec.Code)
	}
}

// failingFS cannot be opened at all.
type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) { return nil, errors.New("disk failure") }

// unreadableFS lists its files but cannot read them.
type unreadableFS struct{ fstest.MapFS }

func (unreadableFS) ReadFile(string) ([]byte, error) { return nil, errors.New("read failure") }

func TestNewHandlerErrors(t *testing.T) {
	tests := []struct {
		name string
		fsys fs.FS
		want string
	}{
		{"unknown extension", fstest.MapFS{"index.html": {}, "assets/logo.png": {}},
			"web: assets/logo.png: no content type for its extension"},
		{"no extension", fstest.MapFS{"LICENSE": {}}, "web: LICENSE: no content type for its extension"},
		{"unopenable", failingFS{}, "web: disk failure"},
		{"unreadable", unreadableFS{fstest.MapFS{"index.html": {}}}, "web: read failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewHandler(tt.fsys)
			if err == nil || err.Error() != tt.want {
				t.Errorf("NewHandler = %v, %v; want error %q", h, err, tt.want)
			}
		})
	}
}

// The embedded build is the dist directory, and the handler accepts whatever
// it holds: only .gitkeep, or a real build copied in before go build.
func TestDist(t *testing.T) {
	if _, err := fs.Stat(Dist(), ".gitkeep"); err != nil {
		t.Errorf("Dist() has no .gitkeep: %v", err)
	}
	newHandler(t, Dist())
}
