// Package spreadsheet opens a workbook version in the user's real spreadsheet
// application (Excel, LibreOffice Calc, or whatever the OS is set up for).
//
// It exists because Argus stores *diffs*, not whole sheets: a reviewer who
// wants to browse the full model — or look at a sheet this commit never
// touched — has to leave for the real tool. This is that door.
//
// Two rules shape the implementation:
//
//   - NEVER open the original file. Selecting a sheet is a WRITE to the
//     workbook, and a historical version is immutable (ROADMAP §7). Every open
//     goes through a read-only temp copy, which also keeps the file out of any
//     folder the daemon is watching — opening a snapshot must not look like a
//     new save and get committed as a version branching off ancient history.
//
//   - Sheet selection happens IN THE FILE, not on the command line. Neither
//     Excel nor LibreOffice accepts an "open at sheet X" argument. But .xlsx
//     records the active tab in xl/workbook.xml, so setting it before launch
//     works in every application that reads the format.
package spreadsheet

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/xuri/excelize/v2"
)

// App is a spreadsheet application found on this machine.
type App struct {
	Name string   // human-readable, shown in the UI ("Microsoft Excel")
	argv []string // launch command; the file path is appended
}

// Opener opens workbook versions. The zero value is ready to use.
type Opener struct {
	// Run launches the application. Nil uses os/exec. Tests inject a stub so
	// they can verify the command without actually opening an app.
	Run func(name string, args ...string) error
	// TempDir overrides where the read-only copy is written. Empty uses the OS
	// temp dir. It must never be a folder argusd watches.
	TempDir string
}

func (o Opener) run(name string, args ...string) error {
	if o.Run != nil {
		return o.Run(name, args...)
	}
	return exec.Command(name, args...).Start()
}

// Detect returns the spreadsheet applications available here, best first.
// Excel is preferred when present because it is the app that wrote the file and
// the one whose rendering the user trusts; LibreOffice is the common fallback.
// The last entry is always the OS default handler, which works even when
// neither is installed (Numbers, Google Sheets via browser, WPS, ...).
func Detect() []App {
	var apps []App
	switch runtime.GOOS {
	case "darwin":
		for _, c := range []struct{ bundle, name string }{
			{"/Applications/Microsoft Excel.app", "Microsoft Excel"},
			{"/Applications/LibreOffice.app", "LibreOffice Calc"},
		} {
			if _, err := os.Stat(c.bundle); err == nil {
				apps = append(apps, App{Name: c.name, argv: []string{"open", "-a", c.bundle}})
			}
		}
		apps = append(apps, App{Name: "default application", argv: []string{"open"}})
	case "windows":
		for _, c := range []struct{ bin, name string }{
			{"excel.exe", "Microsoft Excel"},
			{"soffice.exe", "LibreOffice Calc"},
		} {
			if p, err := exec.LookPath(c.bin); err == nil {
				apps = append(apps, App{Name: c.name, argv: []string{p}})
			}
		}
		// `start` needs an empty title argument before the path.
		apps = append(apps, App{Name: "default application", argv: []string{"cmd", "/c", "start", ""}})
	default: // linux and friends
		for _, c := range []struct{ bin, name string }{
			{"libreoffice", "LibreOffice Calc"},
			{"soffice", "LibreOffice Calc"},
		} {
			if p, err := exec.LookPath(c.bin); err == nil {
				apps = append(apps, App{Name: c.name, argv: []string{p, "--calc"}})
				break
			}
		}
		apps = append(apps, App{Name: "default application", argv: []string{"xdg-open"}})
	}
	return apps
}

// OpenAt copies src to a read-only temp file with `sheet` selected as the
// active tab, opens it in the best available application, and returns that
// application's name.
//
// label names the copy the user will see in their app's title bar — pass
// something that reads as a historical snapshot ("Atlas LBO @ c06"), since it
// is deliberately not the live file. An empty or unknown sheet just opens the
// workbook at whatever tab it already had.
func (o Opener) OpenAt(src, sheet, label string) (string, error) {
	apps := Detect()
	if len(apps) == 0 {
		return "", fmt.Errorf("no spreadsheet application found")
	}
	dst, err := o.materialize(src, sheet, label)
	if err != nil {
		return "", err
	}
	app := apps[0]
	args := append(slices.Clone(app.argv[1:]), dst)
	if err := o.run(app.argv[0], args...); err != nil {
		return "", fmt.Errorf("launching %s: %w", app.Name, err)
	}
	return app.Name, nil
}

// materialize writes the read-only copy and returns its path.
func (o Opener) materialize(src, sheet, label string) (string, error) {
	dir := o.TempDir
	if dir == "" {
		// A dedicated subdirectory, so it is obvious what these files are and
		// trivially excluded from any watch root.
		dir = filepath.Join(os.TempDir(), "argus-open")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}

	dst := filepath.Join(dir, safeName(label)+".xlsx")
	// Remove any previous copy first: it is chmod 0444, so a plain write fails.
	_ = os.Remove(dst)
	if err := copyFile(src, dst); err != nil {
		return "", err
	}

	if sheet != "" {
		if err := setActiveSheet(dst, sheet); err != nil {
			// Selecting the tab is a nicety; failing it should not block the
			// user from getting to their workbook.
			_ = err
		}
	}

	// Read-only on disk, so Excel/LibreOffice open it in read-only mode and the
	// user cannot accidentally "save" edits into a historical snapshot.
	if err := os.Chmod(dst, 0o444); err != nil {
		return "", fmt.Errorf("marking %s read-only: %w", dst, err)
	}
	return dst, nil
}

// setActiveSheet rewrites the copy so `sheet` is the tab shown on open.
func setActiveSheet(path, sheet string) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer f.Close()
	idx, err := f.GetSheetIndex(sheet)
	if err != nil || idx < 0 {
		return fmt.Errorf("sheet %q not found", sheet)
	}
	f.SetActiveSheet(idx)
	return f.Save()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	return out.Close()
}

// safeName makes a label usable as a filename on every platform, preserving
// enough of it to stay recognisable in a title bar.
func safeName(label string) string {
	if strings.TrimSpace(label) == "" {
		label = "workbook"
	}
	var b strings.Builder
	for _, r := range label {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', 0:
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > 80 {
		out = out[:80]
	}
	return strings.TrimRight(out, ". ")
}
