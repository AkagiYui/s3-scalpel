package main

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AppService exposes window management, environment info and OS integrations.
type AppService struct{ core *Core }

// NewWindow opens a new independent window and returns its id.
func (s *AppService) NewWindow() string { return s.core.NewWindow() }

// AppInfo bundles version and environment information for the About section.
type AppInfo struct {
	Version      string `json:"version"`
	BuildVersion string `json:"buildVersion"`
	Debug        bool   `json:"debug"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	DataDir      string `json:"dataDir"`
	CacheDir     string `json:"cacheDir"`
	GoVersion    string `json:"goVersion"`
	WindowCount  int    `json:"windowCount"`
}

// Info returns application and environment information.
func (s *AppService) Info() AppInfo {
	return AppInfo{
		Version:      s.core.version,
		BuildVersion: s.core.buildVer,
		Debug:        s.core.debug,
		Platform:     runtime.GOOS,
		Arch:         runtime.GOARCH,
		DataDir:      s.core.store.BaseDir(),
		CacheDir:     s.core.cacheDir,
		GoVersion:    runtime.Version(),
		WindowCount:  len(s.core.activeWindowIDs()),
	}
}

// OpenURL opens a URL (or mailto:) in the default handler.
func (s *AppService) OpenURL(url string) error {
	if s.core.app == nil {
		return nil
	}
	return s.core.app.Browser.OpenURL(url)
}

// window resolves a window by id for dialog attachment.
func (s *AppService) window(windowID string) application.Window {
	if s.core.app == nil {
		return nil
	}
	if w, ok := s.core.app.Window.Get(windowID); ok {
		return w
	}
	return nil
}

// PickFiles opens a multi-select file chooser (for uploads).
func (s *AppService) PickFiles(windowID string) ([]string, error) {
	d := s.core.app.Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle("Select files to upload")
	if w := s.window(windowID); w != nil {
		d.AttachToWindow(w)
	}
	return d.PromptForMultipleSelection()
}

// PickFolders opens a directory chooser allowing multiple folders (for recursive
// upload).
func (s *AppService) PickFolders(windowID string) ([]string, error) {
	d := s.core.app.Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		SetTitle("Select folders to upload")
	if w := s.window(windowID); w != nil {
		d.AttachToWindow(w)
	}
	return d.PromptForMultipleSelection()
}

// PickDirectory opens a single directory chooser (download destination).
func (s *AppService) PickDirectory(windowID, title string) (string, error) {
	if title == "" {
		title = "Select destination folder"
	}
	d := s.core.app.Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true).
		SetTitle(title)
	if w := s.window(windowID); w != nil {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

// SaveFileAs opens a save dialog for a single download and returns the chosen
// path (empty if cancelled).
func (s *AppService) SaveFileAs(windowID, filename string) (string, error) {
	d := s.core.app.Dialog.SaveFile().
		SetFilename(filename).
		CanCreateDirectories(true).
		SetMessage("Save as")
	if w := s.window(windowID); w != nil {
		d.AttachToWindow(w)
	}
	return d.PromptForSingleSelection()
}

// Confirm shows a native yes/no dialog and returns true if confirmed.
func (s *AppService) Confirm(windowID, title, message string) bool {
	result := make(chan bool, 1)
	dlg := s.core.app.Dialog.Question().
		SetTitle(title).
		SetMessage(message)
	yes := dlg.AddButton("Yes")
	yes.OnClick(func() { result <- true })
	no := dlg.AddButton("No")
	no.OnClick(func() { result <- false })
	no.SetAsCancel()
	if w := s.window(windowID); w != nil {
		dlg.AttachToWindow(w)
	}
	dlg.Show()
	return <-result
}
