package main

import (
	"embed"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// liveStoreMiddleware serves /store/* from a live on-disk directory when
// ARGUS_STORE is set, so the desktop app tracks the daemon's output in real time
// (auto-update, ⌘R, the "new change" toast) instead of the snapshot embedded at
// build time. With the env unset it's a no-op and the bundled snapshot is served
// as before, so a plain `wails build` still runs fully self-contained.
func liveStoreMiddleware(next http.Handler) http.Handler {
	dir := os.Getenv("ARGUS_STORE")
	if dir == "" {
		return next
	}
	files := http.StripPrefix("/store/", http.FileServer(http.Dir(dir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/store/") {
			w.Header().Set("Cache-Control", "no-store")
			files.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Argus",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: liveStoreMiddleware,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
