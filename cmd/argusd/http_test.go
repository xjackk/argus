package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"argus/engine"
)

// seededDaemon builds a daemon with a real three-commit timeline on disk
// (base + two diffs), plus a live httptest server over its API.
func seededDaemon(t *testing.T) (*daemon, *httptest.Server) {
	t.Helper()
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "atlas.xlsx")

	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	copyInto(t, filepath.Join(testdata, "atlas_v1_base.xlsx"), watched)
	d.capture(watched)
	copyInto(t, filepath.Join(testdata, "atlas_v2_exit_multiple.xlsx"), watched)
	d.capture(watched)
	copyInto(t, filepath.Join(testdata, "atlas_v5_hardcode_override.xlsx"), watched)
	d.capture(watched)
	d.writeHistory()

	srv := httptest.NewServer(d.apiHandler())
	t.Cleanup(srv.Close)
	return d, srv
}

func get(t *testing.T, srv *httptest.Server, path string) (int, []byte) {
	t.Helper()
	res, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return res.StatusCode, b
}

func TestAPIVersion(t *testing.T) {
	_, srv := seededDaemon(t)
	code, body := get(t, srv, "/api/version")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	var v struct {
		API   string `json:"api"`
		Major int    `json:"major"`
		Minor int    `json:"minor"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("version does not parse: %v (%s)", err, body)
	}
	if v.API != "1.0" || v.Major != apiMajor || v.Minor != apiMinor {
		t.Errorf("unexpected version payload: %s", body)
	}
}

// /api/commits must decode into exactly the History shape the store file uses,
// so a client parses both with one code path.
func TestAPICommits(t *testing.T) {
	d, srv := seededDaemon(t)
	code, body := get(t, srv, "/api/commits")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	var h History
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("commits do not parse as History: %v", err)
	}
	if len(h.Commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(h.Commits))
	}
	if h.Commits[0].ID != "c001" || !h.Commits[0].Base {
		t.Errorf("first commit should be base c001, got %+v", h.Commits[0])
	}
	if h.Commits[2].Parent != "c002" {
		t.Errorf("c003 parent = %q, want c002", h.Commits[2].Parent)
	}
	if h.Commits[0].Author != d.history.Commits[0].Author {
		t.Errorf("author over HTTP disagrees with the daemon's own record")
	}

	// Every key the client's StoreCommit interface names must be present on
	// the wire — a missing key would silently become undefined in the UI.
	var raw struct {
		Commits []map[string]json.RawMessage `json:"commits"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("raw parse: %v", err)
	}
	for _, k := range []string{"id", "file", "author", "message", "timestamp",
		"parent", "authoredCount", "computedCount", "anomaly", "base"} {
		if _, ok := raw.Commits[0][k]; !ok {
			t.Errorf("commit JSON is missing key %q that store.ts reads", k)
		}
	}
}

func TestAPIDiffByID(t *testing.T) {
	d, srv := seededDaemon(t)

	code, body := get(t, srv, "/api/diff/c002")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	var res engine.DiffResult
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("diff does not parse as engine.DiffResult: %v", err)
	}
	if res.Summary.AuthoredCount != 1 || res.Summary.ComputedCount != 4 {
		t.Errorf("diff counts = %d/%d, want 1/4", res.Summary.AuthoredCount, res.Summary.ComputedCount)
	}

	// The API must serve the same bytes the client would have read off disk.
	onDisk, err := os.ReadFile(filepath.Join(d.store, "diffs", "c002.json"))
	if err != nil {
		t.Fatalf("reading diff from store: %v", err)
	}
	if string(body) != string(onDisk) {
		t.Error("HTTP diff bytes differ from the store file — the two sources must be interchangeable")
	}
}

// A base commit has no diff by design. That must read as a specific 404, not
// as a generic error, so the UI can render "first version".
func TestAPIDiffBaseCommit(t *testing.T) {
	_, srv := seededDaemon(t)
	code, body := get(t, srv, "/api/diff/c001")
	if code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 (body %s)", code, body)
	}
	if !strings.Contains(string(body), "base commit") {
		t.Errorf("base-commit 404 should say so, got %s", body)
	}
}

func TestAPIDiffUnknownCommit(t *testing.T) {
	_, srv := seededDaemon(t)
	if code, body := get(t, srv, "/api/diff/c999"); code != http.StatusNotFound {
		t.Errorf("status %d, want 404 (body %s)", code, body)
	}
}

func TestAPIDiffFromTo(t *testing.T) {
	_, srv := seededDaemon(t)

	// The pair that exists: c001 is c002's parent.
	code, body := get(t, srv, "/api/diff/c001/c002")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	var res engine.DiffResult
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("diff does not parse: %v", err)
	}
	if res.Summary.AuthoredCount != 1 {
		t.Errorf("wrong diff served for c001→c002")
	}

	// A pair nobody computed. Answering would mean recomputing history
	// (ROADMAP §7 invariant 1), so it is a 404 that names the right parent.
	code, body = get(t, srv, "/api/diff/c001/c003")
	if code != http.StatusNotFound {
		t.Errorf("skip-a-commit diff should 404, got %d", code)
	}
	if !strings.Contains(string(body), "c002") {
		t.Errorf("404 should name the actual parent, got %s", body)
	}

	// Backwards.
	if code, _ := get(t, srv, "/api/diff/c002/c001"); code != http.StatusNotFound {
		t.Errorf("reversed pair should 404, got %d", code)
	}
	// Unknown target.
	if code, _ := get(t, srv, "/api/diff/c001/c999"); code != http.StatusNotFound {
		t.Errorf("unknown target should 404, got %d", code)
	}
}

// The whole point of the /store mirror: the client's existing fetches work
// verbatim against the daemon, with the same bytes it would read off disk.
func TestStoreMirrorMatchesDisk(t *testing.T) {
	d, srv := seededDaemon(t)

	code, body := get(t, srv, "/store/history.json")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	onDisk, err := os.ReadFile(filepath.Join(d.store, "history.json"))
	if err != nil {
		t.Fatalf("reading history.json: %v", err)
	}
	if string(body) != string(onDisk) {
		t.Errorf("/store/history.json differs from the file on disk:\nHTTP:\n%s\nDISK:\n%s", body, onDisk)
	}

	code, body = get(t, srv, "/store/diffs/c003.json")
	if code != http.StatusOK {
		t.Fatalf("status %d, body %s", code, body)
	}
	onDisk, err = os.ReadFile(filepath.Join(d.store, "diffs", "c003.json"))
	if err != nil {
		t.Fatalf("reading c003.json: %v", err)
	}
	if string(body) != string(onDisk) {
		t.Error("/store/diffs/c003.json differs from the file on disk")
	}

	// The path shape the client actually builds must not 404 on a miss in a
	// surprising way.
	if code, _ := get(t, srv, "/store/diffs/c001.json"); code != http.StatusNotFound {
		t.Errorf("base commit diff should 404, got %d", code)
	}
}

// Ids reach the filesystem, so they are validated as a strict allowlist.
func TestAPIRejectsPathTraversal(t *testing.T) {
	d, srv := seededDaemon(t)
	// A file the daemon writes one directory above diffs/ — the obvious target.
	secret := filepath.Join(d.store, "history.json")
	if _, err := os.Stat(secret); err != nil {
		t.Fatalf("expected history.json to exist: %v", err)
	}

	for _, path := range []string{
		"/api/diff/..%2f..%2fhistory",
		"/api/diff/..%2fhistory",
		"/store/diffs/..%2fhistory.json",
		"/api/diff/%2Fetc%2Fpasswd",
		"/api/diff/c002%00",
		"/api/diff/.",
	} {
		code, body := get(t, srv, path)
		if code == http.StatusOK {
			t.Errorf("GET %s returned 200 — traversal not blocked (%s)", path, body)
		}
		if strings.Contains(string(body), "\"commits\"") {
			t.Errorf("GET %s leaked history.json", path)
		}
	}

	if validCommitID("../history") || validCommitID("c002.json") || validCommitID("") || validCommitID("a/b") {
		t.Error("validCommitID accepted an id that can escape store/diffs/")
	}
	if !validCommitID("c001") || !validCommitID("c1000") {
		t.Error("validCommitID rejected a legitimate id")
	}
}

// Read-only: nothing on the API accepts a write.
func TestAPIIsReadOnly(t *testing.T) {
	_, srv := seededDaemon(t)
	for _, p := range []string{"/api/commits", "/api/diff/c002", "/store/history.json"} {
		res, err := srv.Client().Post(srv.URL+p, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("POST %s: %v", p, err)
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Errorf("POST %s returned 200 — the API must be read-only", p)
		}
	}
}

// Cross-origin GET must be allowed: the client is on a different origin from
// the daemon in every deployment that turns -http on.
func TestAPIAllowsCrossOriginReads(t *testing.T) {
	_, srv := seededDaemon(t)
	res, err := srv.Client().Get(srv.URL + "/api/commits")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
	// Credentials must NOT be allowed alongside a wildcard origin.
	if res.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("wildcard CORS must not also allow credentials")
	}
}

// Concurrency: many readers hitting the API while the daemon captures. Run
// under -race, this is the guard on d.history/lastCommit/lastSnapshot/seq.
func TestAPIConcurrentReadsDuringCapture(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "atlas.xlsx")

	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	srv := httptest.NewServer(d.apiHandler())
	defer srv.Close()

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				res, err := srv.Client().Get(srv.URL + "/api/commits")
				if err != nil {
					return
				}
				b, _ := io.ReadAll(res.Body)
				res.Body.Close()
				// A reader must NEVER see a truncated or invalid timeline,
				// no matter when it lands relative to a capture.
				var h History
				if err := json.Unmarshal(b, &h); err != nil {
					t.Errorf("reader got unparseable history mid-capture: %v", err)
					return
				}
				for _, c := range h.Commits {
					if c.ID == "" {
						t.Error("reader saw a commit with no id")
						return
					}
				}
				// And the store file itself must always be whole.
				if fb, err := os.ReadFile(filepath.Join(store, "history.json")); err == nil {
					var fh History
					if err := json.Unmarshal(fb, &fh); err != nil {
						t.Errorf("history.json was read half-written: %v", err)
						return
					}
				}
			}
		}()
	}

	// Capture concurrently with all that reading.
	for _, f := range []string{
		"atlas_v1_base.xlsx",
		"atlas_v2_exit_multiple.xlsx",
		"atlas_v3_downside.xlsx",
		"atlas_v5_hardcode_override.xlsx",
	} {
		copyInto(t, filepath.Join(testdata, f), watched)
		d.capture(watched)
		d.writeHistory()
	}

	close(stop)
	readers.Wait()

	if got := len(d.historySnapshot().Commits); got != 4 {
		t.Fatalf("expected 4 commits after concurrent run, got %d", got)
	}
}

// historySnapshot must hand out a copy — a caller mutating it (or ranging it
// while capture appends) must not touch the daemon's own slice.
func TestHistorySnapshotIsACopy(t *testing.T) {
	d, _ := seededDaemon(t)
	snap := d.historySnapshot()
	snap.Commits[0].Author = "mallory"
	if d.history.Commits[0].Author == "mallory" {
		t.Error("historySnapshot returned a view into the daemon's own slice")
	}
}

// No -http means no listener, ever.
func TestServeHTTPIsNoOpWithoutAddr(t *testing.T) {
	d := newDaemon(t.TempDir(), t.TempDir(), "alice")
	if err := d.serveHTTP(); err != nil {
		t.Fatalf("serveHTTP with no address should be a silent no-op, got %v", err)
	}
}

// An -http address that cannot be bound is fatal: the operator asked for it.
func TestServeHTTPFailsLoudlyOnBadAddr(t *testing.T) {
	d := newDaemon(t.TempDir(), t.TempDir(), "alice")
	d.httpAddr = "256.256.256.256:1"
	if err := d.serveHTTP(); err == nil {
		t.Error("serveHTTP should return an error when the address cannot be bound")
	}
}
