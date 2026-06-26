// Package webui embeds the compiled buktio web panel (a static Next.js export)
// into the Go binary and serves it as a single-page application. This is what
// lets the OSS `buktio-api` and the paid `buktio-api-ee` ship as ONE binary with
// no separate Node web container — both import this package via the shared
// internal/httpapi router, so they always carry an identical UI artifact.
//
// The embedded directory `dist` is populated by `make web-embed` (which runs the
// web static export and copies apps/web/out here). A placeholder index.html is
// committed so the package always compiles even before the UI is built; the real
// export overwrites the whole directory. See internal/webui/dist/.gitignore.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// all:dist is required (not just `dist`): go:embed skips files and directories
// whose names begin with "_" or ".", and the Next export keeps its assets under
// _next/, which would otherwise be omitted.
//
//go:embed all:dist
var distFS embed.FS

// placeholderHTML is served when no real UI export has been embedded yet (only the
// committed placeholder.html is present). Keeps a bare API-only binary usable and
// makes the missing-build state obvious instead of returning a blank 404.
const placeholderHTML = `<!doctype html><meta charset="utf-8"><title>buktio</title>` +
	`<body style="font-family:system-ui;margin:3rem;color:#333">` +
	`<h1>buktio</h1><p>The web panel was not embedded in this build. ` +
	`Run <code>make web-embed</code> (or use an official release image). ` +
	`The API is available under <code>/api/v1</code>.</p></body>`

// Embedded reports whether a real UI export is present (as opposed to the
// committed placeholder). Used by the guard test and /readyz diagnostics.
func Embedded() bool {
	// The real export ships hashed assets under dist/_next; the placeholder does not.
	entries, err := fs.ReadDir(distFS, "dist/_next")
	return err == nil && len(entries) > 0
}

// Handler returns an http.Handler serving the embedded SPA. Existing files are
// served directly with appropriate cache headers; any unmatched, non-asset path
// falls back to index.html (HTTP 200) so client-side routing and the query-param
// detail routes resolve correctly on deep links and hard refreshes.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		// No real export embedded — fall back to the placeholder notice.
		index = []byte(placeholderHTML)
	}
	return &spaHandler{
		fsys:       sub,
		fileServer: http.FileServer(http.FS(sub)),
		index:      index,
	}, nil
}

type spaHandler struct {
	fsys       fs.FS
	fileServer http.Handler
	index      []byte
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := strings.TrimPrefix(r.URL.Path, "/")
	if upath == "" {
		upath = "index.html"
	}

	if h.exists(upath) || h.exists(strings.TrimSuffix(upath, "/")+"/index.html") {
		// Hashed build assets are content-addressed and safe to cache forever;
		// everything else (html shells) must stay fresh so deploys are picked up.
		if strings.HasPrefix(upath, "_next/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// SPA fallback: serve the app shell so client-side routing takes over.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.index)
}

func (h *spaHandler) exists(name string) bool {
	if name == "" {
		return false
	}
	f, err := h.fsys.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
