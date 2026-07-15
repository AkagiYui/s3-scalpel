package main

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"s3scalpel/internal/model"
	"s3scalpel/internal/queue"
	"s3scalpel/internal/s3x"
	"s3scalpel/internal/store"
)

const (
	settingsFile = "settings.json"
	connsFile    = "connections.enc"
)

// Core holds all shared backend state. A single instance is shared by every
// window because Wails runs one Go process for all webviews.
type Core struct {
	store    *store.Store
	cacheDir string
	clients  *s3x.Manager
	queue    *queue.Manager

	app   *application.App
	notif *notifications.NotificationService

	settingsMu sync.RWMutex
	settings   model.AppSettings

	connMu sync.RWMutex
	conns  []model.Connection

	version  string
	buildVer string
	debug    bool

	// notifyOK is false when system notifications are unsafe to use — notably a
	// bare (un-bundled) macOS binary, where UNUserNotificationCenter aborts the
	// process. This keeps `wails3 dev` (which runs the bare binary) from crashing.
	notifyOK bool
}

// NewCore wires up persistence, the S3 client cache and the queue manager.
func NewCore(dataDir, cacheDir, version, buildVer string, debug bool) (*Core, error) {
	st, err := store.New(dataDir)
	if err != nil {
		return nil, err
	}
	c := &Core{
		store:    st,
		cacheDir: cacheDir,
		clients:  s3x.NewManager(),
		version:  version,
		buildVer: buildVer,
		debug:    debug,
		settings: defaultSettings(),
		notifyOK: notificationsSafe(),
	}
	c.loadSettings()
	c.loadConnections()

	c.queue = queue.NewManager(st, queue.Deps{
		GetConnection: c.getConnection,
		GetClient:     c.getClient,
		Emit:          c.emit,
		Settings:      c.Settings,
		Notify:        c.Notify,
	})
	return c, nil
}

func defaultSettings() model.AppSettings {
	return model.AppSettings{
		Language:         "system",
		Theme:            "system",
		NotifyEnabled:    true,
		NotifySound:      true,
		Concurrency:      5,
		PartSize:         8 * 1024 * 1024,
		PartConcurrency:  4,
		MultipartEnabled: true,
		AutoConsumeQueue: true,
		PreviewMaxSize:   10 * 1024 * 1024,
	}
}

// SetApp attaches the Wails application after it is created.
func (c *Core) SetApp(app *application.App, notif *notifications.NotificationService) {
	c.app = app
	c.notif = notif
}

// Settings returns a copy of the current settings.
func (c *Core) Settings() model.AppSettings {
	c.settingsMu.RLock()
	defer c.settingsMu.RUnlock()
	return c.settings
}

func (c *Core) loadSettings() {
	s := defaultSettings()
	if _, err := c.store.ReadJSON(settingsFile, &s); err == nil {
		if s.Concurrency <= 0 {
			s.Concurrency = 5
		}
		if s.PartSize < s3x.MinPartSize {
			s.PartSize = s3x.MinPartSize
		}
		if s.PreviewMaxSize <= 0 {
			s.PreviewMaxSize = 10 * 1024 * 1024
		}
		if s.PartConcurrency <= 0 {
			s.PartConcurrency = 4
		}
		c.settings = s
	}
}

func (c *Core) saveSettings() error {
	c.settingsMu.RLock()
	s := c.settings
	c.settingsMu.RUnlock()
	return c.store.WriteJSON(settingsFile, s)
}

func (c *Core) loadConnections() {
	var conns []model.Connection
	if _, err := c.store.ReadEncrypted(connsFile, &conns); err == nil {
		c.conns = conns
	}
}

func (c *Core) saveConnections() error {
	c.connMu.RLock()
	conns := make([]model.Connection, len(c.conns))
	copy(conns, c.conns)
	c.connMu.RUnlock()
	return c.store.WriteEncrypted(connsFile, conns)
}

func (c *Core) getConnection(id string) (model.Connection, bool) {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	for _, conn := range c.conns {
		if conn.ID == id {
			return conn, true
		}
	}
	return model.Connection{}, false
}

func (c *Core) getClient(ctx context.Context, conn model.Connection) (*s3.Client, error) {
	return c.clients.Client(ctx, conn)
}

// clientFor resolves a connection id to an S3 client.
func (c *Core) clientFor(ctx context.Context, connID string) (*s3.Client, model.Connection, error) {
	conn, ok := c.getConnection(connID)
	if !ok {
		return nil, model.Connection{}, errConnNotFound
	}
	cl, err := c.getClient(ctx, conn)
	return cl, conn, err
}

// emit sends an event to all windows (no-op until the app is attached).
func (c *Core) emit(event string, data any) {
	if c.app != nil {
		c.app.Event.Emit(event, data)
	}
}

// Notify sends a system notification (if enabled) and an in-app sound cue (if
// enabled). The two settings are honoured independently.
func (c *Core) Notify(title, body string, isError bool) {
	s := c.Settings()
	if !s.NotifyEnabled {
		return
	}
	if c.notif != nil && c.notifyOK {
		_ = c.notif.SendNotification(notifications.NotificationOptions{
			ID:    randID(),
			Title: title,
			Body:  body,
		})
	}
	if s.NotifySound {
		c.emit("notify:sound", map[string]any{"error": isError})
	}
}

// NotifyOK reports whether system notifications can be used safely.
func (c *Core) NotifyOK() bool { return c.notifyOK }

// notificationsSafe reports whether the OS notification APIs are safe to call.
// On macOS this requires running from inside a .app bundle; the bare binary
// (e.g. under `wails3 dev`) aborts when touching UNUserNotificationCenter.
func notificationsSafe() bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(exe, ".app/Contents/MacOS/")
}

var errConnNotFound = &appError{"connection not found"}

type appError struct{ msg string }

func (e *appError) Error() string { return e.msg }
