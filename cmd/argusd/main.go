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
// Usage:
//
//	argusd [-folder DIR] [-store DIR] [-author NAME]
//
// Defaults: folder=~/ArgusDropbox, store=./desktop/frontend/public/store
// (so the dev client serves it at /store), author=the OS user.
package main

import (
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
	"strings"
	"sync"
	"time"

	"argus/engine"

	"github.com/fsnotify/fsnotify"
)

// Commit is one captured version in the store's timeline.
type Commit struct {
	ID             string `json:"id"`
	File           string `json:"file"`
	Author         string `json:"author"`
	Message        string `json:"message"`
	Timestamp      string `json:"timestamp"`
	Parent         string `json:"parent"` // previous commit id for the same file, "" if base
	AuthoredCount  int    `json:"authoredCount"`
	ComputedCount  int    `json:"computedCount"`
	Anomaly        bool   `json:"anomaly"`
	Base           bool   `json:"base"` // first version of this file (no diff)
}

// History is the store manifest the client reads.
type History struct {
	Commits []Commit `json:"commits"`
}

type daemon struct {
	folder string
	store  string
	author string

	mu           sync.Mutex
	history      History
	lastCommit   map[string]string // file key -> last commit id
	lastSnapshot map[string]string // file key -> last snapshot path
	seq          int
}

func main() {
	home, _ := os.UserHomeDir()
	folder := flag.String("folder", filepath.Join(home, "ArgusDropbox"), "watched folder")
	store := flag.String("store", filepath.Join("desktop", "frontend", "public", "store"), "store output dir (client reads this)")
	author := flag.String("author", osUser(), "commit author")
	flag.Parse()

	d := &daemon{
		folder:       *folder,
		store:        *store,
		author:       *author,
		lastCommit:   map[string]string{},
		lastSnapshot: map[string]string{},
	}
	if err := d.run(); err != nil {
		log.Fatalf("argusd: %v", err)
	}
}

func (d *daemon) run() error {
	for _, dir := range []string{d.folder, d.store, filepath.Join(d.store, "diffs"), filepath.Join(d.store, "versions")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	log.Printf("argus daemon — watching %s", d.folder)
	log.Printf("            store  %s   author %q", d.store, d.author)
	log.Printf("Drop .xlsx files into the watched folder (or save over one) to track them.")

	// Snapshot any files already sitting in the folder as their base version.
	entries, _ := os.ReadDir(d.folder)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if isXlsx(e.Name()) {
			d.capture(filepath.Join(d.folder, e.Name()))
		}
	}
	d.writeHistory()

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
		if !isXlsx(path) || isTemp(path) {
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
			if _, err := os.Stat(path); err == nil {
				d.capture(path)
				d.writeHistory()
			}
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
			log.Printf("watch error: %v", err)
		}
	}
}

// capture snapshots one file and, if it has a prior version, diffs and commits.
func (d *daemon) capture(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := fileKey(path)
	name := filepath.Base(path)

	// Copy the current file into the versions store (retry: it may be briefly
	// locked mid-save by the editor).
	snapDir := filepath.Join(d.store, "versions", key)
	os.MkdirAll(snapDir, 0o755)
	d.seq++
	snap := filepath.Join(snapDir, fmt.Sprintf("%06d.xlsx", d.seq))
	if err := copyFileRetry(path, snap, 5); err != nil {
		log.Printf("skip %s: %v", name, err)
		d.seq--
		return
	}

	prevSnap, hadPrev := d.lastSnapshot[key]
	// Ignore no-op saves (content identical to the last snapshot).
	if hadPrev && sameContent(prevSnap, snap) {
		os.Remove(snap)
		d.seq--
		return
	}

	id := fmt.Sprintf("c%03d", len(d.history.Commits)+1)
	commit := Commit{
		ID:        id,
		File:      name,
		Author:    d.author,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Parent:    d.lastCommit[key],
	}

	if !hadPrev {
		commit.Base = true
		commit.Message = "Added " + name
		log.Printf("＋ %s  tracked %s (base version)", id, name)
	} else {
		res, err := engine.Diff(prevSnap, snap)
		if err != nil {
			log.Printf("diff %s: %v", name, err)
		} else {
			commit.AuthoredCount = res.Summary.AuthoredCount
			commit.ComputedCount = res.Summary.ComputedCount
			commit.Anomaly = len(res.Anomalies) > 0
			d.writeDiff(id, res)
		}
		commit.Message = fmt.Sprintf("Updated %s", name)
		badge := ""
		if commit.Anomaly {
			badge = " ⚠ anomaly"
		}
		log.Printf("● %s  %s — %d authored · %d computed%s", id, name, commit.AuthoredCount, commit.ComputedCount, badge)
	}

	d.history.Commits = append(d.history.Commits, commit)
	d.lastSnapshot[key] = snap
	d.lastCommit[key] = id
}

func (d *daemon) writeDiff(id string, res engine.DiffResult) {
	b, _ := json.MarshalIndent(res, "", "  ")
	os.WriteFile(filepath.Join(d.store, "diffs", id+".json"), b, 0o644)
}

func (d *daemon) writeHistory() {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, _ := json.MarshalIndent(d.history, "", "  ")
	os.WriteFile(filepath.Join(d.store, "history.json"), b, 0o644)
}

// --- helpers ---

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

// isTemp filters editor lock/temp files (Excel/LibreOffice write ~$foo, .~lock).
func isTemp(path string) bool {
	b := filepath.Base(path)
	return strings.HasPrefix(b, "~$") || strings.HasPrefix(b, ".~") || strings.HasPrefix(b, ".")
}

func fileKey(path string) string {
	sum := sha1.Sum([]byte(filepath.Base(path)))
	return hex.EncodeToString(sum[:6])
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
	_, err = io.Copy(out, in)
	return err
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
