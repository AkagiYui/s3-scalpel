package queue

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
	"s3scalpel/internal/store"
)

// testDeps returns deps that never actually run S3 work (GetClient errors), with
// auto-consume off so queued tasks stay pending and are easy to assert on.
func testDeps() Deps {
	return Deps{
		GetConnection: func(id string) (model.Connection, bool) {
			return model.Connection{ID: id}, true
		},
		GetClient: func(ctx context.Context, c model.Connection) (*s3.Client, error) {
			return nil, context.Canceled
		},
		Emit:     func(string, any) {},
		Settings: func() model.AppSettings { return model.AppSettings{Concurrency: 3, AutoConsumeQueue: false} },
		Notify:   func(string, string, bool) {},
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(st, testDeps())
}

func TestRecoveryMarksRunningAsFailed(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a previous session that crashed mid-run.
	persisted := []*model.Task{
		{ID: "a", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusRunning},
		{ID: "b", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusPending},
		{ID: "c", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusCompleted},
	}
	if err := st.WriteJSON(tasksFile, persisted); err != nil {
		t.Fatal(err)
	}

	m := NewManager(st, testDeps())
	tasks := m.Tasks("win-1")
	byID := map[string]model.Task{}
	for _, tk := range tasks {
		byID[tk.ID] = tk
	}
	if byID["a"].Status != model.StatusFailed {
		t.Errorf("running task should be recovered as failed, got %s", byID["a"].Status)
	}
	if byID["b"].Status != model.StatusPending {
		t.Errorf("pending task should stay pending, got %s", byID["b"].Status)
	}
	if byID["c"].Status != model.StatusCompleted {
		t.Errorf("completed task should stay completed, got %s", byID["c"].Status)
	}
}

func TestAddAndControlOps(t *testing.T) {
	m := newTestManager(t)
	wid := "win-1"

	for i, p := range []int{0, 5, 1} {
		m.Add(&model.Task{
			ID: string(rune('x' + i)), WindowID: wid, Type: model.TaskDelete,
			Status: model.StatusPending, Priority: p,
		})
	}
	if got := len(m.Tasks(wid)); got != 3 {
		t.Fatalf("want 3 tasks, got %d", got)
	}

	// Remove a pending task.
	m.Remove(wid, "x")
	if got := len(m.Tasks(wid)); got != 2 {
		t.Fatalf("after remove want 2, got %d", got)
	}

	// SetPriority then verify it persists in the snapshot.
	m.SetPriority(wid, "y", 9)
	for _, tk := range m.Tasks(wid) {
		if tk.ID == "y" && tk.Priority != 9 {
			t.Errorf("priority not updated: %d", tk.Priority)
		}
	}

	// ClearFinished should not drop pending tasks.
	m.ClearFinished(wid)
	if got := len(m.Tasks(wid)); got != 2 {
		t.Errorf("ClearFinished dropped pending tasks: %d", got)
	}
}

func TestStateReflectsControls(t *testing.T) {
	m := newTestManager(t)
	wid := "win-2"
	m.SetConcurrency(wid, 7)
	m.SetAutoConsume(wid, true)
	st := m.State(wid)
	if st["concurrency"].(int) != 7 {
		t.Errorf("concurrency=%v", st["concurrency"])
	}
	if st["autoConsume"].(bool) != true {
		t.Errorf("autoConsume=%v", st["autoConsume"])
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	m := NewManager(st, testDeps())
	m.Add(&model.Task{ID: "z", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusPending})
	m.Flush()

	// A fresh manager over the same store should see the task.
	time.Sleep(10 * time.Millisecond)
	m2 := NewManager(st, testDeps())
	if got := len(m2.Tasks("win-1")); got != 1 {
		t.Errorf("persisted task not reloaded, got %d", got)
	}
}
