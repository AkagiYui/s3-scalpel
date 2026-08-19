package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

// VERSION is the single source of truth for the application version; a unit test
// keeps build/config.yml in step with it. COMMIT identifies the exact build and
// is rewritten by CI (to the short SHA or the release tag) just before packaging;
// it stays "dev" for local builds.
//
//go:embed VERSION
var versionRaw string

//go:embed COMMIT
var commitRaw string

var (
	version      = strings.TrimSpace(versionRaw)
	buildVersion = strings.TrimSpace(commitRaw)
)

const appName = "S3 Scalpel"

func main() {
	dataDir, cacheDir := appDirs()

	core, err := NewCore(dataDir, cacheDir, version, buildVersion, buildDebug)
	if err != nil {
		log.Fatal(err)
	}

	services := []application.Service{
		application.NewService(&SettingsService{core: core}),
		application.NewService(&ConfigService{core: core}),
		application.NewService(&S3Service{core: core}),
		application.NewService(&BucketService{core: core}),
		application.NewService(&QueueService{core: core}),
		application.NewService(&PreviewService{core: core}),
		application.NewService(&BookmarkService{core: core}),
		application.NewService(&AppService{core: core}),
	}

	// The macOS notifications service requires a valid app bundle to start. Only
	// register it when running bundled, so the bare-binary dev workflow doesn't
	// abort at startup.
	var notif *notifications.NotificationService
	if core.NotifyOK() {
		notif = notifications.New()
		services = append(services, application.NewService(notif))
	}

	app := application.New(application.Options{
		Name:        appName,
		Description: "A surgical S3-compatible object storage client",
		Services:    services,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// Serves staged image/PDF previews from /_preview/, so they stream
			// instead of being inlined as base64.
			Middleware: core.previews.middleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			core.queue.Flush()
			core.session.saveNow()
			core.previews.discardAll()
		},
	})

	core.SetApp(app, notif)
	app.Menu.SetApplicationMenu(buildMenu(app, core))

	// Request notification permission when the service is active.
	if notif != nil {
		go func() {
			_, _ = notif.RequestNotificationAuthorization()
		}()
	}

	// First window.
	core.createWindow(nextWindowName())

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// appDirs resolves platform-standard data and cache directories for the app.
func appDirs() (dataDir, cacheDir string) {
	cfgBase, err := os.UserConfigDir()
	if err != nil || cfgBase == "" {
		cfgBase, _ = os.MkdirTemp("", "s3scalpel-cfg")
	}
	cacheBase, err := os.UserCacheDir()
	if err != nil || cacheBase == "" {
		cacheBase = cfgBase
	}
	dataDir = filepath.Join(cfgBase, "S3Scalpel")
	cacheDir = filepath.Join(cacheBase, "S3Scalpel")
	return dataDir, cacheDir
}
