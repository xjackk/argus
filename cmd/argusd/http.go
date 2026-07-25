package main

// The read-only HTTP API (ROADMAP §6.2).
//
// This is served ALONGSIDE the store the daemon has always written, never
// instead of it. history.json and diffs/*.json keep landing on disk exactly as
// before; these handlers are a second door onto the same data. Nothing here
// mutates the store, so a client that cannot reach the API — or a deployment
// that never turns it on — is in exactly the state it was in before.
//
// Two surfaces are exposed:
//
//   - /api/…    the ROADMAP §6.2 shapes, for anything written against the API
//     as an API.
//   - /store/…  a byte-compatible mirror of the on-disk store layout, so the
//     existing client can be pointed at a remote daemon by changing one base
//     URL and nothing else — same paths, same JSON, no parsing changes.
//
// Deliberately NOT here: POST /api/commits, GET /api/events (SSE), and
// /api/blob/{hash}. See the report — writes and content-addressed blobs need
// the commit store from §6.1, which does not exist yet.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// apiVersion is the contract version for the §6.6 handshake. Bump the major
// on any breaking change to the shapes below; bump the minor for additions.
const (
	apiMajor = 1
	apiMinor = 0
)

// serveHTTP starts the read-only API if -http was given, and is a no-op
// otherwise. It returns an error only when a listener was asked for and could
// not be created — the operator explicitly requested a port, so silently
// running without one would hide the failure until a client timed out.
func (d *daemon) serveHTTP() error {
	if d.httpAddr == "" {
		return nil
	}
	ln, err := net.Listen("tcp", d.httpAddr)
	if err != nil {
		return fmt.Errorf("http listen on %s: %w", d.httpAddr, err)
	}
	log.Printf("            http   %s   (api v%d.%d — /api/commits, /api/diff/{id}, /store/*)",
		ln.Addr(), apiMajor, apiMinor)
	srv := &http.Server{Handler: d.apiHandler()}
	go func() {
		// The watch loop owns the process lifetime; if the server ever stops,
		// log it and leave the daemon capturing to disk. Losing the API must
		// never take down capture.
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("! http server stopped: %v", err)
		}
	}()
	return nil
}

// apiHandler builds the router. Split out from serveHTTP so tests can drive it
// through httptest without binding a port.
func (d *daemon) apiHandler() http.Handler {
	mux := http.NewServeMux()

	// --- ROADMAP §6.2 ---
	mux.HandleFunc("GET /api/version", d.handleVersion)
	mux.HandleFunc("GET /api/commits", d.handleCommits)
	mux.HandleFunc("GET /api/diff/{id}", d.handleDiffByID)
	mux.HandleFunc("GET /api/diff/{from}/{to}", d.handleDiffFromTo)

	// --- store-compatible mirror ---
	// Same paths and same bytes as the on-disk store, so the client's
	// `${STORE}/history.json` and `${STORE}/diffs/${id}.json` work verbatim
	// against a remote daemon.
	mux.HandleFunc("GET /store/history.json", d.handleStoreHistory)
	mux.HandleFunc("GET /store/diffs/{file}", d.handleStoreDiff)

	return withReadOnlyCORS(mux)
}

// withReadOnlyCORS allows cross-origin GETs.
//
// Needed because the whole point of -http is a client on a different origin
// from the daemon (the Vite dev server on :5173, a Wails app on wails://, an
// analyst's laptop talking to a box in the VPC). The surface it opens is
// exactly the read-only data below — there is no write endpoint and no
// credential to steal, since the server has no auth to be confused about.
// It intentionally does not set Allow-Credentials.
//
// If argusd ever grows POST /api/commits or per-device keys (ROADMAP §6.4),
// this must become an allowlist. Flagged in the report.
func withReadOnlyCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// --- handlers ---

// GET /api/version — the client↔server handshake (ROADMAP §6.6).
func (d *daemon) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"api":           fmt.Sprintf("%d.%d", apiMajor, apiMinor),
		"major":         apiMajor,
		"minor":         apiMinor,
		"schemaVersion": storeSchemaVersion,
	})
}

// GET /api/commits — the timeline, oldest first.
//
// Served from memory (the authoritative copy) rather than by re-reading
// history.json, so a reader never observes the gap between a commit being
// appended and the file being rewritten. The shape is the same History object
// the file holds, so a client can parse both with one code path.
func (d *daemon) handleCommits(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, d.historySnapshot())
}

// GET /api/diff/{id} — one commit's DiffResult (engine/types.go, unchanged).
func (d *daemon) handleDiffByID(w http.ResponseWriter, r *http.Request) {
	d.serveDiff(w, r.PathValue("id"))
}

// GET /api/diff/{from}/{to} — the §6.2 two-sided form.
//
// The store is a linear per-file chain, so the only diff that exists is
// parent→child: {to}'s recorded diff, valid exactly when {from} is {to}'s
// parent. Any other pair is a diff nobody has computed, and inventing one here
// would mean recomputing history — ROADMAP §7 invariant 1 forbids that. So it
// is a 404 with a body that says which parent would have worked.
func (d *daemon) handleDiffFromTo(w http.ResponseWriter, r *http.Request) {
	from, to := r.PathValue("from"), r.PathValue("to")
	c, ok := d.commitByID(to)
	if !ok {
		httpError(w, http.StatusNotFound, "no commit %q", to)
		return
	}
	if c.Parent != from {
		httpError(w, http.StatusNotFound,
			"no stored diff %s→%s; %s's parent is %q (Argus stores parent→child diffs only and never recomputes history)",
			from, to, to, c.Parent)
		return
	}
	d.serveDiff(w, to)
}

// GET /store/history.json — the mirror. Byte-identical to the file on disk:
// same MarshalIndent settings, same object.
func (d *daemon) handleStoreHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, d.historySnapshot())
}

// GET /store/diffs/{id}.json — the mirror.
func (d *daemon) handleStoreDiff(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	id, ok := strings.CutSuffix(file, ".json")
	if !ok {
		httpError(w, http.StatusNotFound, "not found")
		return
	}
	d.serveDiff(w, id)
}

// serveDiff streams a commit's diff file straight from the store.
//
// It serves the bytes rather than decode/re-encode, so what a client gets over
// HTTP is exactly what it would have got reading the file — including the
// narrative, once the background narrator has rewritten it. Because
// writeDiff publishes via atomic rename, a read here always lands on one whole
// version of the file, never a half-written one.
func (d *daemon) serveDiff(w http.ResponseWriter, id string) {
	if !validCommitID(id) {
		httpError(w, http.StatusBadRequest, "bad commit id %q", id)
		return
	}
	b, err := os.ReadFile(filepath.Join(d.store, "diffs", id+".json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Either the id does not exist, or it is a base commit (which by
			// design has no diff) or a save whose diff failed at capture.
			// Distinguish, so the client can render "first version" rather
			// than an error.
			if c, ok := d.commitByID(id); ok && c.Base {
				httpError(w, http.StatusNotFound, "%s is a base commit and has no diff", id)
				return
			}
			httpError(w, http.StatusNotFound, "no diff for %q", id)
			return
		}
		log.Printf("! http: reading diff %s: %v", id, err)
		httpError(w, http.StatusInternalServerError, "could not read diff %q", id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

// --- helpers ---

// storeSchemaVersion mirrors engine.DiffResult.SchemaVersion, reported in the
// handshake so a client can refuse a store it cannot read.
const storeSchemaVersion = 1

// validCommitID gates every id that reaches the filesystem. Ids the daemon
// mints are "c001"-shaped, but the id in a URL is attacker-controlled, so the
// check is a strict allowlist rather than a traversal blocklist: no dots, no
// separators, nothing that can escape store/diffs/.
func validCommitID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// writeJSON encodes with the same indentation the store files use, so the
// mirror endpoints are byte-comparable with the files on disk.
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("! http: encoding response: %v", err)
		http.Error(w, `{"error":"encoding failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func httpError(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}
