package main

import (
	"sync"
	"testing"
	"time"

	"s3scalpel/internal/model"
)

// memStore is an in-memory storeWriter so session tests never touch the disk.
type memStore struct {
	mu   sync.Mutex
	data map[string]any
}

func newMemStore() *memStore { return &memStore{data: map[string]any{}} }

func (m *memStore) ReadJSON(name string, v any) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.data[name]
	if !ok {
		return false, nil
	}
	if state, isState := stored.(sessionState); isState {
		if target, isTarget := v.(*sessionState); isTarget {
			*target = state
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) WriteJSON(name string, v any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := v.(sessionState); ok {
		m.data[name] = state
	}
	return nil
}

func (m *memStore) saved() (sessionState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.data[sessionFile].(sessionState)
	return state, ok
}

func TestSessionRoundTripsTabsAndBounds(t *testing.T) {
	st := newMemStore()
	s := newSession(st)

	tabs := []model.TabSession{
		{ConnectionID: "c1", Title: "MinIO", Bucket: "photos", Prefix: "2024/", Active: true},
		{ConnectionID: "c2", Title: "R2", Bucket: "", Prefix: ""},
	}
	s.setTabs("win-1", tabs)
	s.setBounds("win-1", model.WindowBounds{X: 40, Y: 60, Width: 1400, Height: 900})
	s.saveNow()

	saved, ok := st.saved()
	if !ok {
		t.Fatal("nothing was persisted")
	}
	got := saved.Windows["win-1"]
	if len(got.Tabs) != 2 || got.Tabs[0].Bucket != "photos" || !got.Tabs[0].Active {
		t.Errorf("tabs round-tripped as %+v", got.Tabs)
	}
	if got.Bounds.Width != 1400 || got.Bounds.X != 40 {
		t.Errorf("bounds round-tripped as %+v", got.Bounds)
	}

	// A fresh session over the same store sees the persisted workspace.
	restored := newSession(st).get("win-1")
	if len(restored.Tabs) != 2 || restored.Bounds.Height != 900 {
		t.Errorf("restore produced %+v", restored)
	}
}

func TestSessionIgnoresDegenerateBounds(t *testing.T) {
	s := newSession(newMemStore())
	s.setBounds("win-1", model.WindowBounds{X: 0, Y: 0, Width: 1200, Height: 800})
	// A minimised window reports a tiny (or zero) rect; restoring that would
	// reopen the app as an unusable sliver.
	s.setBounds("win-1", model.WindowBounds{X: 0, Y: 0, Width: 0, Height: 0})

	if got := s.get("win-1").Bounds; got.Width != 1200 || got.Height != 800 {
		t.Errorf("bounds = %+v, want the last usable geometry", got)
	}
}

func TestSessionKeepsWindowsIndependent(t *testing.T) {
	s := newSession(newMemStore())
	s.setTabs("win-1", []model.TabSession{{ConnectionID: "a"}})
	s.setTabs("win-2", []model.TabSession{{ConnectionID: "b"}, {ConnectionID: "c"}})

	if got := s.get("win-1").Tabs; len(got) != 1 || got[0].ConnectionID != "a" {
		t.Errorf("win-1 tabs = %+v", got)
	}
	if got := s.get("win-2").Tabs; len(got) != 2 {
		t.Errorf("win-2 tabs = %+v", got)
	}
	if got := s.get("win-9"); len(got.Tabs) != 0 || got.Bounds.Width != 0 {
		t.Errorf("an unknown window should be zero-valued, got %+v", got)
	}
}

func TestSessionDebouncesWrites(t *testing.T) {
	st := newMemStore()
	s := newSession(st)

	// A drag emits a move event per frame; those must coalesce into one write.
	for i := range 20 {
		s.setBounds("win-1", model.WindowBounds{Width: 1000 + i, Height: 800})
	}
	if _, ok := st.saved(); ok {
		t.Fatal("bounds were written before the debounce elapsed")
	}
	time.Sleep(700 * time.Millisecond)

	saved, ok := st.saved()
	if !ok {
		t.Fatal("the debounced write never happened")
	}
	if saved.Windows["win-1"].Bounds.Width != 1019 {
		t.Errorf("persisted width = %d, want the final 1019", saved.Windows["win-1"].Bounds.Width)
	}
}
