package main

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// buildMenu constructs the application menu. Shortcut items emit events targeted
// at the focused window so the frontend can act on its own tab state.
func buildMenu(app *application.App, core *Core) *application.Menu {
	menu := application.NewMenu()

	emit := func(action string) func(*application.Context) {
		return func(*application.Context) {
			wid := ""
			if w := app.Window.Current(); w != nil {
				wid = w.Name()
			}
			app.Event.Emit("menu:action", map[string]any{"action": action, "wid": wid})
		}
	}

	if runtime.GOOS == "darwin" {
		appMenu := menu.AddSubmenu("S3 Scalpel")
		appMenu.Add("About S3 Scalpel").OnClick(emit("about"))
		appMenu.AddSeparator()
		appMenu.Add("Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(emit("settings"))
		appMenu.AddSeparator()
		appMenu.AddRole(application.Hide)
		appMenu.AddRole(application.HideOthers)
		appMenu.AddRole(application.UnHide)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Quit)
	}

	file := menu.AddSubmenu("File")
	file.Add("New Window").SetAccelerator("CmdOrCtrl+Shift+N").OnClick(func(*application.Context) {
		core.NewWindow()
	})
	file.Add("New Connection").SetAccelerator("CmdOrCtrl+N").OnClick(emit("new-connection"))
	file.Add("New Tab").SetAccelerator("CmdOrCtrl+T").OnClick(emit("new-tab"))
	file.Add("Close Tab").SetAccelerator("CmdOrCtrl+W").OnClick(emit("close-tab"))
	if runtime.GOOS != "darwin" {
		file.AddSeparator()
		file.Add("Settings").SetAccelerator("CmdOrCtrl+,").OnClick(emit("settings"))
		file.AddSeparator()
		file.AddRole(application.Quit)
	}

	edit := menu.AddSubmenu("Edit")
	edit.AddRole(application.Undo)
	edit.AddRole(application.Redo)
	edit.AddSeparator()
	edit.AddRole(application.Cut)
	edit.AddRole(application.Copy)
	edit.AddRole(application.Paste)
	edit.AddRole(application.SelectAll)

	view := menu.AddSubmenu("View")
	view.Add("Reload").SetAccelerator("CmdOrCtrl+R").OnClick(func(*application.Context) {
		if w := app.Window.Current(); w != nil {
			w.Reload()
		}
	})
	view.Add("Refresh List").SetAccelerator("CmdOrCtrl+Shift+R").OnClick(emit("refresh"))
	view.Add("Find").SetAccelerator("CmdOrCtrl+F").OnClick(emit("find"))
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)
	if core.debug {
		view.Add("Toggle DevTools").SetAccelerator("CmdOrCtrl+Alt+I").OnClick(func(*application.Context) {
			if w := app.Window.Current(); w != nil {
				w.OpenDevTools()
			}
		})
	}

	window := menu.AddSubmenu("Window")
	window.AddRole(application.Minimise)
	window.AddRole(application.Zoom)

	help := menu.AddSubmenu("Help")
	help.Add("Homepage (aky.moe)").OnClick(func(*application.Context) {
		_ = app.Browser.OpenURL("https://aky.moe")
	})

	return menu
}
