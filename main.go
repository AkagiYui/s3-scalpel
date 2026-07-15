package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version metadata. buildVersion can be overridden at link time with
// -ldflags "-X main.buildVersion=<sha>".
var (
	version      = "0.1.0"
	buildVersion = "dev"
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
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			core.queue.Flush()
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
