package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Attribution — who actually saved this version.
//
// The daemon's original model is one process-level -author stamped on every
// commit. That is correct for the solo "Dropbox model" (one person, one
// laptop, one daemon) and flatly wrong for a server: one argusd watching a
// shared folder for a team would credit every save in the firm to whoever the
// service happens to run as.
//
// The fix is to ask the workbook. Every .xlsx carries docProps/core.xml, and
// Excel — and LibreOffice, and Google Sheets on export — writes the saving
// user's display name into <cp:lastModifiedBy> on every save. excelize reads
// it via GetDocProps().LastModifiedBy (ROADMAP §5.1).
//
// The chain is:
//
//	docProps.LastModifiedBy  →  -author flag  →  the OS user
//
// # TRUST MODEL — read this before believing an author field
//
// This is METADATA, NOT AUTHENTICATION. LastModifiedBy is a plain string that
// the client application writes into a zip entry. It is:
//
//   - self-reported — it is whatever display name that copy of Excel is
//     configured with, which the user sets themselves and can change freely;
//   - trivially forgeable — anyone who can write a file into the watched
//     folder can put any name in it, with a text editor and a zip tool;
//   - unverified — nothing links it to a session, a device, or an account,
//     and the daemon has no way to check it;
//   - often absent — machine-written workbooks (openpyxl, pandas, most
//     report generators) leave it empty. The bundled atlas_* fixtures do.
//
// So it answers "whose Excel most recently wrote these bytes, according to
// that Excel" — which is genuinely useful attribution for a cooperating team,
// and worthless as an access-control or audit primitive. Anything that needs
// to be defensible (compliance, sign-off, information barriers per ROADMAP §8)
// needs the per-device key exchange in ROADMAP §6.4, not this. Do not present
// this value in the UI as a verified identity.

// authorFor resolves the author for the version stored at snapshot.
//
// It reads the workbook's own docProps.LastModifiedBy when
// -attribute-from-file is on, and falls back to the process-level -author
// (which itself defaults to the OS user) whenever that is unavailable, empty,
// or the mode is off. The fallback means this can only ever add information —
// with the flag off, or on a workbook with no such property, the answer is
// byte-for-byte what the daemon produced before.
func (d *daemon) authorFor(snapshot string) string {
	if !d.attributeFromFile {
		return d.author
	}
	if who := workbookLastModifiedBy(snapshot); who != "" {
		return who
	}
	return d.author
}

// workbookLastModifiedBy returns the trimmed docProps.LastModifiedBy of an
// .xlsx, or "" if the file is unreadable, is not a workbook, has no document
// properties, or the property is blank.
//
// It never returns an error and never panics: attribution is a nice-to-have on
// the capture path, and no failure to read a name may ever cost us the commit.
// excelize parses attacker-shaped zip/XML here, so the recover is load-bearing,
// not defensive decoration.
func workbookLastModifiedBy(path string) (who string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("· attribution: recovered while reading %s: %v", filepath.Base(path), r)
			who = ""
		}
	}()

	f, err := excelize.OpenFile(path)
	if err != nil {
		return "" // not a workbook, locked, corrupt — fall back, don't fail
	}
	defer f.Close()

	props, err := f.GetDocProps()
	if err != nil || props == nil {
		return ""
	}
	return strings.TrimSpace(props.LastModifiedBy)
}
