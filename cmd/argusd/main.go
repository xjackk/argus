// Command argusd is the Argus capture daemon — the "Dropbox model" for Excel.
//
// It is the SINGLE watcher and SINGLE writer: it watches one folder, and on
// every .xlsx save it snapshots the file, diffs it against that file's previous
// snapshot via the engine, and appends a commit to a store on disk. The client
// never touches the raw files — it only reads (and polls/subscribes to) the
// store the daemon writes. Run the daemon on a laptop for single-user, or on a
// server for a team; the architecture is identical, only the store location and
// transport change.
//
// The daemon is designed to never crash on bad input: a file that vanishes
// mid-capture, a locked/corrupt .xlsx, a diff-engine error, or a store write
// failure are all logged and skipped — the watch loop stays alive. On restart
// it RESUMES the existing store (rebuilding its in-memory state from
// history.json + the versions on disk) so relaunching — e.g. with a different
// -author — continues the timeline instead of overwriting it.
//
// Usage:
//
//	argusd [-folder DIR] [-store DIR] [-author NAME]
//	       [-http ADDR] [-attribute-from-file]
//
// Defaults: folder=~/ArgusDropbox, store=./desktop/frontend/public/store
// (so the dev client serves it at /store), author=the OS user.
//
// # Server mode
//
// With -http the daemon ALSO serves a read-only HTTP API over the very same
// store it writes (see http.go). This is strictly additive: the files on disk
// are written exactly as before, and with no -http flag not a single byte of
// behaviour changes. The API reads what the daemon already produced; it is not
// an alternative storage path.
package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"argus/engine"
	"argus/narrator"

	"github.com/fsnotify/fsnotify"
)

// Commit is one captured version in the store's timeline.
type Commit struct {
	ID            string `json:"id"`
	File          string `json:"file"`
	Author        string `json:"author"`
	Message       string `json:"message"`
	Timestamp     string `json:"timestamp"`
	Parent        string `json:"parent"` // previous commit id for the same file, "" if base
	AuthoredCount int    `json:"authoredCount"`
	ComputedCount int    `json:"computedCount"`
	Anomaly       bool   `json:"anomaly"`
	Base          bool   `json:"base"` // first version of this file (no diff)
}

// History is the store manifest the client reads.
type History struct {
	Commits []Commit `json:"commits"`
}

type daemon struct {
	folder  string
	store   string
	author  string            // -author: the process-level fallback identity
	narrate narrator.Narrator // fills summary.narrative per diff; nil = skip (tests)

	// httpAddr, when non-empty, is the address the read-only HTTP API listens
	// on (see http.go). Empty — the default — means no listener is ever
	// created and the daemon behaves exactly as it always has.
	httpAddr string

	// attributeFromFile turns on per-save attribution from the workbook's own
	// docProps.LastModifiedBy (see attribution.go). Off by default so the
	// no-flag path is byte-identical; defaults ON when -http is set, because a
	// server watching one folder for a whole team is precisely the case where
	// a single process-level -author is wrong.
	attributeFromFile bool

	// mu guards history/lastCommit/lastSnapshot/seq. It is an RWMutex so that
	// concurrent HTTP readers can share it without serialising behind each
	// other (only capture and writeHistory take the write lock).
	mu           sync.RWMutex
	history      History
	lastCommit   map[string]string // file key -> last commit id
	lastSnapshot map[string]string // file key -> last snapshot path
	seq          int
}

// newDaemon builds a daemon with initialized state. It does not touch the disk;
// call run (or the individual ensureDirs/resume/scanFolder steps in tests).
func newDaemon(folder, store, author string) *daemon {
	return &daemon{
		folder:       folder,
		store:        store,
		author:       author,
		lastCommit:   map[string]string{},
		lastSnapshot: map[string]string{},
	}
}

func main() {
	home, _ := os.UserHomeDir()
	folder := flag.String("folder", filepath.Join(home, "ArgusDropbox"), "watched folder")
	store := flag.String("store", filepath.Join("desktop", "frontend", "public", "store"), "store output dir (client reads this)")
	author := flag.String("author", osUser(), "commit author")
	narrate := flag.Bool("narrate", true, "fill each commit's plain-English summary via `claude -p` (grounded, async ~2-3s)")
	model := flag.String("model", "", "model for narration (default: claude CLI default)")
	httpAddr := flag.String("http", "", "also serve the read-only HTTP API on this `address` (e.g. :7777); empty = off, the default")
	attribute := flag.Bool("attribute-from-file", false, "attribute each commit to the workbook's own docProps.LastModifiedBy, falling back to -author (defaults ON when -http is set)")
	flag.Parse()

	// -attribute-from-file defaults ON in server mode. One daemon serving a
	// team over HTTP stamping every commit with one -author is the exact bug
	// this fixes; but flipping the default unconditionally would change what
	// the existing filesystem-only path writes, so the flip is scoped to the
	// opt-in mode. An explicit -attribute-from-file=false still wins.
	attributeFromFile := *attribute
	if *httpAddr != "" && !flagWasSet("attribute-from-file") {
		attributeFromFile = true
	}

	d := newDaemon(*folder, *store, *author)
	d.httpAddr = *httpAddr
	d.attributeFromFile = attributeFromFile
	if *narrate {
		// Fallback → Noop so a missing/failed claude CLI degrades to a null
		// narrative instead of erroring the capture.
		d.narrate = narrator.Fallback{
			// 45s: narration runs async (nothing waits on it), and a big workbook
			// with a big change can make for a long prompt — well past the 8s
			// ClaudeCLI default — so give it plenty of room rather than silently
			// falling back to an empty summary.
			Primary: narrator.ClaudeCLI{Model: *model, Timeout: 45 * time.Second},
			Backup:  narrator.Noop{},
		}
	}
	if err := d.run(); err != nil {
		log.Fatalf("argusd: %v", err)
	}
}

func (d *daemon) run() error {
	if err := d.ensureDirs(); err != nil {
		return err
	}
	log.Printf("argus daemon — watching %s", d.folder)
	log.Printf("            store  %s   author %q", d.store, d.author)
	if d.attributeFromFile {
		log.Printf("            attribution: docProps.LastModifiedBy, falling back to %q", d.author)
	}

	// Resume any existing store so a restart continues the timeline.
	if err := d.resume(); err != nil {
		// A resume failure is not fatal: log loudly and start a fresh timeline
		// rather than refusing to run.
		log.Printf("! could not resume existing store: %v — starting fresh", err)
	}

	// Start the read-only API alongside — never instead of — the file writing.
	// Started after ensureDirs/resume so the first request already sees a
	// complete store. A bad -http address is fatal: the operator explicitly
	// asked for a listener, so silently continuing without one would be worse
	// than refusing to start.
	if err := d.serveHTTP(); err != nil {
		return err
	}

	log.Printf("Drop .xlsx files into the watched folder (or save over one) to track them.")

	// Snapshot any files already sitting in the folder. Files already tracked
	// and unchanged (content identical to their last snapshot) are skipped, so
	// a restart does not re-add every file as a new base commit.
	d.scanFolder()
	d.writeHistory()

	return d.watch()
}

// ensureDirs creates the folder + store layout the daemon writes into.
func (d *daemon) ensureDirs() error {
	for _, dir := range []string{d.folder, d.store, filepath.Join(d.store, "diffs"), filepath.Join(d.store, "versions")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}

// resume loads an existing store into memory so a restart continues the
// timeline instead of overwriting it. It rebuilds:
//   - history         from store/history.json
//   - lastCommit[key] the latest commit id per file
//   - lastSnapshot[key] the latest snapshot path under store/versions/<key>/
//   - seq             the highest snapshot number seen on disk
//
// It is a no-op (nil) when there is no history yet, and it degrades gracefully
// (logs, starts fresh) if history.json is present but corrupt.
func (d *daemon) resume() error {
	histPath := filepath.Join(d.store, "history.json")
	b, err := os.ReadFile(histPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh store — nothing to resume
		}
		return fmt.Errorf("reading %s: %w", histPath, err)
	}

	var h History
	if err := json.Unmarshal(b, &h); err != nil {
		// Corrupt history must not crash the daemon; start a fresh timeline.
		log.Printf("! history.json is unreadable (%v) — starting a fresh timeline", err)
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.history = h

	// lastCommit: commits are appended in order, so the last one for a file wins.
	keys := map[string]bool{}
	for _, c := range h.Commits {
		key := fileKey(c.File)
		d.lastCommit[key] = c.ID
		keys[key] = true
	}

	// lastSnapshot + seq: read what is actually on disk under versions/<key>/.
	for key := range keys {
		snap, n := latestSnapshot(filepath.Join(d.store, "versions", key))
		if snap != "" {
			d.lastSnapshot[key] = snap
		}
		if n > d.seq {
			d.seq = n
		}
	}

	log.Printf("resumed %d commit(s) across %d file(s) — next id c%03d, seq at %06d",
		len(h.Commits), len(keys), len(h.Commits)+1, d.seq)

	// Backfill summaries for any commit whose diff has none — e.g. commits
	// captured before narration was wired, or while it was failing. Runs in the
	// background so startup isn't blocked, and the client picks each up on its
	// next poll. A copy of the slice is passed so this never races the watch loop.
	if d.narrate != nil {
		go d.backfillNarratives(append([]Commit(nil), h.Commits...))
	}
	return nil
}

// backfillNarratives fills summaries for already-captured diffs that have none.
// It reads each commit's diff and, if summary.narrative is empty, narrates and
// rewrites it. Sequential so a restart isn't a burst of model calls; best-effort,
// so a failure just leaves that diff's narrative null.
func (d *daemon) backfillNarratives(commits []Commit) {
	for _, c := range commits {
		if c.Base {
			continue // base commits have no diff
		}
		b, err := os.ReadFile(filepath.Join(d.store, "diffs", c.ID+".json"))
		if err != nil {
			continue // no diff on disk (e.g. the diff failed at capture) — skip
		}
		var res engine.DiffResult
		if err := json.Unmarshal(b, &res); err != nil {
			continue
		}
		if res.Summary.Narrative != nil && *res.Summary.Narrative != "" {
			continue // already summarized
		}
		log.Printf("… backfilling summary for %s", c.ID)
		d.narrateInto(c.ID, res) // inline within this goroutine (already async of run)
	}
}

// scanFolder captures every .xlsx currently in the watched folder. New files
// become base commits; already-tracked unchanged files are skipped by capture.
func (d *daemon) scanFolder() {
	entries, err := os.ReadDir(d.folder)
	if err != nil {
		log.Printf("! could not scan folder %s: %v", d.folder, err)
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !isXlsx(e.Name()) || isTemp(e.Name()) {
			continue
		}
		d.capture(filepath.Join(d.folder, e.Name()))
	}
}

// watch runs the fsnotify event loop until the watcher closes. Errors are
// logged and the loop keeps running — the daemon never exits on a bad event.
func (d *daemon) watch() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	if err := w.Add(d.folder); err != nil {
		return err
	}

	// Debounce: editors save via several events + an atomic rename, so coalesce
	// bursts per path and process after things settle.
	pending := map[string]*time.Timer{}
	var pmu sync.Mutex
	schedule := func(path string) {
		if !isXlsx(path) {
			return
		}
		if isTemp(path) {
			// Excel/LibreOffice write ~$foo.xlsx / .~lock files next to the doc.
			return
		}
		pmu.Lock()
		defer pmu.Unlock()
		if t, ok := pending[path]; ok {
			t.Stop()
		}
		pending[path] = time.AfterFunc(800*time.Millisecond, func() {
			pmu.Lock()
			delete(pending, path)
			pmu.Unlock()
			if _, err := os.Stat(path); err != nil {
				log.Printf("· skip %s — gone before capture (%v)", filepath.Base(path), err)
				return
			}
			d.capture(path)
			d.writeHistory()
		})
	}

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) != 0 {
				schedule(ev.Name)
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("watch error: %v", err) // logged — loop stays alive
		}
	}
}

// capture snapshots one file and, if it has a prior version, diffs and commits.
// It is fully self-contained and panic-safe: any panic while handling one file
// is recovered and logged, so a single bad file can never take down the daemon.
func (d *daemon) capture(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("● skip %s — recovered from panic: %v", filepath.Base(path), r)
		}
	}()

	key := fileKey(path)
	name := filepath.Base(path)

	// Copy the current file into the versions store (retry: it may be briefly
	// locked mid-save by the editor).
	snapDir := filepath.Join(d.store, "versions", key)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		log.Printf("· skip %s — cannot create store dir: %v", name, err)
		return
	}
	d.seq++
	snap := filepath.Join(snapDir, fmt.Sprintf("%06d.xlsx", d.seq))
	if err := copyFileRetry(path, snap, 5); err != nil {
		log.Printf("· skip %s — could not read/copy file: %v", name, err)
		d.seq--
		return
	}

	prevSnap, hadPrev := d.lastSnapshot[key]
	// Ignore no-op saves (content identical to the last snapshot). This is also
	// what makes an unchanged, already-tracked file a no-op on restart instead
	// of a duplicate base commit.
	if hadPrev && sameContent(prevSnap, snap) {
		os.Remove(snap)
		d.seq--
		log.Printf("· skip %s — no change since last snapshot", name)
		return
	}

	// Who saved it. Read from the snapshot we just took, never the live file:
	// the original may still be locked by the editor mid-save, and the
	// snapshot is the exact bytes this commit records.
	author := d.authorFor(snap)

	id := fmt.Sprintf("c%03d", len(d.history.Commits)+1)
	commit := Commit{
		ID:        id,
		File:      name,
		Author:    author,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Parent:    d.lastCommit[key],
	}

	if !hadPrev {
		commit.Base = true
		commit.Message = "Added " + name
		log.Printf("＋ %s  tracked %s (base version) — author %q", id, name, author)
	} else {
		res, err := engine.Diff(prevSnap, snap)
		if err != nil {
			// The file copied fine but the engine could not diff it (corrupt or
			// unsupported workbook). Record the save, log the reason, carry on.
			log.Printf("● %s  %s — diff failed (%v); recording save without a diff", id, name, err)
		} else {
			commit.AuthoredCount = res.Summary.AuthoredCount
			commit.ComputedCount = res.Summary.ComputedCount
			commit.Anomaly = len(res.Anomalies) > 0
			// Write the diff immediately with a null narrative so the commit
			// appears instantly (the "poof"). The plain-English summary is filled
			// in the background and the diff rewritten — the client shows a
			// loading pulse until it lands.
			d.writeDiff(id, res)
			if d.narrate != nil {
				go d.narrateInto(id, res)
			}
		}
		commit.Message = fmt.Sprintf("Updated %s", name)
		badge := ""
		if commit.Anomaly {
			badge = " ⚠ anomaly"
		}
		log.Printf("● %s  %s — %d authored · %d computed%s — author %q", id, name, commit.AuthoredCount, commit.ComputedCount, badge, author)
	}

	d.history.Commits = append(d.history.Commits, commit)
	d.lastSnapshot[key] = snap
	d.lastCommit[key] = id
}

// narrateInto fills summary.narrative for an already-written diff via the
// grounded narrator, then rewrites the diff file. Runs in its own goroutine so
// the ~2-3s model call never delays the commit; res is a value copy and the only
// field mutated is Summary.Narrative, so this races with nothing. A failed or
// slow call is logged and leaves the narrative null (the client just stops
// pulsing). Safe to fire per commit — ids are unique, so no two writes collide.
func (d *daemon) narrateInto(id string, res engine.DiffResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	text, err := d.narrate.Narrate(ctx, res)
	if err != nil {
		log.Printf("· narration skipped for %s: %v", id, err)
		return
	}
	if text == "" {
		return
	}
	res.Summary.Narrative = &text
	d.writeDiff(id, res)
	log.Printf("✎ %s  summary ready", id)
}

func (d *daemon) writeDiff(id string, res engine.DiffResult) {
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Printf("! could not encode diff %s: %v", id, err)
		return
	}
	if err := writeFileAtomic(filepath.Join(d.store, "diffs", id+".json"), b, 0o644); err != nil {
		log.Printf("! could not write diff %s: %v", id, err)
	}
}

func (d *daemon) writeHistory() {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, err := json.MarshalIndent(d.history, "", "  ")
	if err != nil {
		log.Printf("! could not encode history: %v", err)
		return
	}
	if err := writeFileAtomic(filepath.Join(d.store, "history.json"), b, 0o644); err != nil {
		log.Printf("! could not write history.json: %v", err)
	}
}

// historySnapshot returns a copy of the in-memory timeline, safe to hand to a
// reader (an HTTP handler) that runs concurrently with capture.
func (d *daemon) historySnapshot() History {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return History{Commits: append([]Commit(nil), d.history.Commits...)}
}

// commitByID looks up one commit in the in-memory timeline.
func (d *daemon) commitByID(id string) (Commit, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, c := range d.history.Commits {
		if c.ID == id {
			return c, true
		}
	}
	return Commit{}, false
}

// --- helpers ---

// writeFileAtomic writes b to path so that a concurrent reader — an HTTP
// handler here, the client's fetch of /store/history.json in the browser —
// only ever sees the complete old file or the complete new one, never a
// truncated one. os.WriteFile truncates in place, which leaves a window where
// history.json is valid JSON's worth of nothing.
//
// The temp file is created in the destination directory so the rename is a
// same-filesystem operation, which POSIX makes atomic.
func writeFileAtomic(path string, b []byte, perm os.FileMode) error {
	dir, base := filepath.Dir(path), filepath.Base(path)
	f, err := os.CreateTemp(dir, "."+base+".tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	// Any failure past this point must not leave the partial file behind.
	defer func() {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	// Flush to the platter before the rename, so a crash cannot publish a name
	// that points at unwritten bytes.
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	tmp = "" // published — do not remove
	return nil
}

// flagWasSet reports whether name was given explicitly on the command line, as
// opposed to sitting at its default. Used to tell "the operator chose false"
// from "nobody said", which is what lets -http flip a default safely.
func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func osUser() string {
	if u, err := user.Current(); err == nil {
		if u.Name != "" {
			return u.Name
		}
		return u.Username
	}
	return "unknown"
}

func isXlsx(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".xlsx")
}

// isTemp filters editor lock/temp files (Excel/LibreOffice write ~$foo, .~lock)
// and any dotfile.
func isTemp(path string) bool {
	b := filepath.Base(path)
	return strings.HasPrefix(b, "~$") || strings.HasPrefix(b, ".~") || strings.HasPrefix(b, ".")
}

func fileKey(path string) string {
	sum := sha1.Sum([]byte(filepath.Base(path)))
	return hex.EncodeToString(sum[:6])
}

// latestSnapshot returns the highest-numbered %06d.xlsx snapshot in dir and its
// numeric sequence. Returns ("", 0) if the dir is missing or has no snapshots.
func latestSnapshot(dir string) (string, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0
	}
	best := -1
	var bestName string
	for _, e := range entries {
		if e.IsDir() || !isXlsx(e.Name()) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if err != nil {
			continue
		}
		if n > best {
			best = n
			bestName = e.Name()
		}
	}
	if best < 0 {
		return "", 0
	}
	return filepath.Join(dir, bestName), best
}

func copyFileRetry(src, dst string, tries int) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = copyFile(src, dst); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func sameContent(a, b string) bool {
	ha, err1 := hashFile(a)
	hb, err2 := hashFile(b)
	return err1 == nil && err2 == nil && ha == hb
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
