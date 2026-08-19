package main

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"s3scalpel/internal/model"
)

const sessionFile = "session.json"

// sessionState is what survives a restart: where each window was on screen and
// which tabs it had open. Windows are keyed by their generated name ("win-1"),
// which is assigned in the same order every run, so the first window reopens the
// first window's workspace.
type sessionState struct {
	Windows map[string]model.WindowSession `json:"windows"`
}

// session owns the persisted workspace, debouncing writes because window moves
// and resizes arrive continuously while the user drags.
type session struct {
	store storeWriter
	mu    sync.Mutex
	state sessionState
	timer *time.Timer
}

// storeWriter is the slice of *store.Store the session needs, kept narrow so the
// type is trivial to fake in tests.
type storeWriter interface {
	ReadJSON(name string, v any) (bool, error)
	WriteJSON(name string, v any) error
}

func newSession(st storeWriter) *session {
	s := &session{store: st, state: sessionState{Windows: map[string]model.WindowSession{}}}
	var loaded sessionState
	if ok, err := st.ReadJSON(sessionFile, &loaded); err == nil && ok && loaded.Windows != nil {
		s.state = loaded
	}
	return s
}

// get returns the stored workspace for a window ("", zero bounds when unknown).
func (s *session) get(windowID string) model.WindowSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Windows[windowID]
}

// setTabs records the tab strip of a window.
func (s *session) setTabs(windowID string, tabs []model.TabSession) {
	s.mu.Lock()
	cur := s.state.Windows[windowID]
	cur.Tabs = tabs
	s.state.Windows[windowID] = cur
	s.mu.Unlock()
	s.scheduleSave()
}

// setBounds records the on-screen geometry of a window.
func (s *session) setBounds(windowID string, b model.WindowBounds) {
	// A minimised or otherwise degenerate window would restore unusably small.
	if b.Width < 200 || b.Height < 200 {
		return
	}
	s.mu.Lock()
	cur := s.state.Windows[windowID]
	cur.Bounds = b
	s.state.Windows[windowID] = cur
	s.mu.Unlock()
	s.scheduleSave()
}

// scheduleSave debounces persistence; drags emit a move event per frame.
func (s *session) scheduleSave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(500*time.Millisecond, s.saveNow)
}

// saveNow writes the workspace immediately (also called on shutdown).
func (s *session) saveNow() {
	s.mu.Lock()
	snapshot := sessionState{Windows: make(map[string]model.WindowSession, len(s.state.Windows))}
	for k, v := range s.state.Windows {
		snapshot.Windows[k] = v
	}
	s.mu.Unlock()
	_ = s.store.WriteJSON(sessionFile, snapshot)
}

// trackWindow wires a window's move/resize events into the session store and
// applies any geometry remembered from the last run.
func (c *Core) trackWindow(w *application.WebviewWindow, name string) {
	if saved := c.session.get(name).Bounds; saved.Width > 0 && saved.Height > 0 {
		w.SetBounds(application.Rect{
			X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height,
		})
	}
	record := func(*application.WindowEvent) {
		b := w.Bounds()
		c.session.setBounds(name, model.WindowBounds{
			X: b.X, Y: b.Y, Width: b.Width, Height: b.Height,
		})
	}
	w.OnWindowEvent(events.Common.WindowDidResize, record)
	w.OnWindowEvent(events.Common.WindowDidMove, record)
}
