package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// titleBarHeight is the height (px) of the frontend-drawn title bar; it matches
// the macOS invisible inset region so the whole visible bar is draggable.
const titleBarHeight = 40

var windowCounter atomic.Int64

const windowPrefix = "win-"

// nextWindowName returns a unique window name like "win-1".
func nextWindowName() string {
	return fmt.Sprintf("%s%d", windowPrefix, windowCounter.Add(1))
}

// NewWindow creates a new application window with its own queue context. The
// window id is passed to the frontend via the URL query string.
func (c *Core) NewWindow() string {
	name := nextWindowName()
	c.createWindow(name)
	return name
}

// createWindow builds a window with the standard chrome and options. The title
// bar is drawn by the frontend: on macOS the native inset traffic lights are
// kept (non-frameless) with a reserved gap; on Windows/Linux the window is
// frameless and the frontend renders its own min/max/close controls.
func (c *Core) createWindow(name string) *application.WebviewWindow {
	frameless := runtime.GOOS != "darwin"
	w := c.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             name,
		Title:            "S3 Scalpel",
		Width:            1200,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		URL:              "/?wid=" + name,
		EnableFileDrop:   true,
		DevToolsEnabled:  c.debug,
		Frameless:        frameless,
		BackgroundColour: application.NewRGB(255, 255, 255),
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: titleBarHeight,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			DisableFramelessWindowDecorations: false,
		},
	})

	// Forward native file drops (onto elements marked data-file-drop-target) to
	// the frontend, tagged with this window's id.
	w.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		c.emit("files:dropped", map[string]any{"wid": name, "paths": files})
	})

	return w
}

// activeWindowIDs returns the names of all currently open app windows.
func (c *Core) activeWindowIDs() []string {
	if c.app == nil {
		return nil
	}
	var ids []string
	for _, w := range c.app.Window.GetAll() {
		if n := w.Name(); strings.HasPrefix(n, windowPrefix) {
			ids = append(ids, n)
		}
	}
	return ids
}
