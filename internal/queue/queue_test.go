package queue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"

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

func TestClassifyTreatsWrappedCancellationAsCanceled(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want model.TaskStatus
	}{
		{"success", nil, model.StatusCompleted},
		{"bare cancel", context.Canceled, model.StatusCanceled},
		// The AWS SDK returns cancellation wrapped in an operation error.
		{"wrapped cancel", fmt.Errorf("operation error S3: PutObject: %w", context.Canceled), model.StatusCanceled},
		{"deeply wrapped", fmt.Errorf("a: %w", fmt.Errorf("b: %w", context.Canceled)), model.StatusCanceled},
		{"deadline", context.DeadlineExceeded, model.StatusFailed},
		{"other", errors.New("connection reset"), model.StatusFailed},
	}
	for _, c := range cases {
		if got := classify(c.err); got != c.want {
			t.Errorf("%s: classify(%v)=%q want %q", c.name, c.err, got, c.want)
		}
	}
}

func TestApplySettingsUpdatesLiveQueues(t *testing.T) {
	m := newTestManager(t)
	m.Queue("win-1")
	m.Queue("win-2")

	m.ApplySettings(9, true)

	for _, wid := range []string{"win-1", "win-2"} {
		state := m.State(wid)
		if state["concurrency"] != 9 {
			t.Errorf("%s concurrency = %v, want 9", wid, state["concurrency"])
		}
		if state["autoConsume"] != true {
			t.Errorf("%s autoConsume = %v, want true", wid, state["autoConsume"])
		}
	}

	// A window created afterwards still picks the values up from Settings().
	m.ApplySettings(0, false)
	if got := m.State("win-1")["concurrency"]; got != 1 {
		t.Errorf("concurrency floor = %v, want 1", got)
	}
}

func TestClassifyRecognisesSkip(t *testing.T) {
	if got := classify(errSkipped); got != model.StatusSkipped {
		t.Errorf("classify(errSkipped) = %q, want %q", got, model.StatusSkipped)
	}
	if got := classify(fmt.Errorf("upload: %w", errSkipped)); got != model.StatusSkipped {
		t.Errorf("a wrapped skip should still classify as skipped, got %q", got)
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{6, 30 * time.Second},
		{40, 30 * time.Second},
	}
	for _, c := range cases {
		if got := backoff(c.attempt); got != c.want {
			t.Errorf("backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestRetriableTaskIsRequeuedWithBackoffThenGivesUp(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	deps := testDeps()
	deps.Settings = func() model.AppSettings {
		return model.AppSettings{Concurrency: 1, AutoConsumeQueue: true, MaxAutoRetries: 1}
	}
	deps.GetClient = func(context.Context, model.Connection) (*s3.Client, error) {
		attempts.Add(1)
		// A 503 is transient, so the queue should retry it.
		return nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 503}},
			Err:      errors.New("service unavailable"),
		}
	}
	m := NewManager(st, deps)

	m.Add(&model.Task{ID: "t1", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusPending})

	deadline := time.Now().Add(10 * time.Second)
	var final model.Task
	for time.Now().Before(deadline) {
		tasks := m.Tasks("win-1")
		if len(tasks) == 1 && tasks[0].Status == model.StatusFailed {
			final = tasks[0]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if final.Status != model.StatusFailed {
		t.Fatalf("task never reached a terminal failure: %+v", m.Tasks("win-1"))
	}
	if final.Retries != 1 {
		t.Errorf("retries = %d, want exactly the configured 1", final.Retries)
	}
	if got := attempts.Load(); got != 2 {
		t.Errorf("executed %d times, want 2 (initial attempt plus one retry)", got)
	}
}

func TestNonRetriableTaskFailsImmediately(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	deps := testDeps()
	deps.Settings = func() model.AppSettings {
		return model.AppSettings{Concurrency: 1, AutoConsumeQueue: true, MaxAutoRetries: 5}
	}
	deps.GetClient = func(context.Context, model.Connection) (*s3.Client, error) {
		attempts.Add(1)
		// 403 will never succeed on retry.
		return nil, &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 403}},
			Err:      errors.New("access denied"),
		}
	}
	m := NewManager(st, deps)
	m.Add(&model.Task{ID: "t1", WindowID: "win-1", Type: model.TaskUpload, Status: model.StatusPending})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tasks := m.Tasks("win-1")
		if len(tasks) == 1 && tasks[0].Status == model.StatusFailed {
			if tasks[0].Retries != 0 {
				t.Errorf("retries = %d, want 0 for a permanent error", tasks[0].Retries)
			}
			if got := attempts.Load(); got != 1 {
				t.Errorf("executed %d times, want 1", got)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task never failed: %+v", m.Tasks("win-1"))
}

func TestPickNextSkipsTasksInBackoff(t *testing.T) {
	m := newTestManager(t)
	q := m.Queue("win-1")
	q.mu.Lock()
	q.autoConsume = true
	q.tasks["waiting"] = &model.Task{
		ID: "waiting", WindowID: "win-1", Status: model.StatusPending,
		NextAttemptAt: time.Now().Add(time.Hour).UnixMilli(),
	}
	q.order = append(q.order, "waiting")
	q.mu.Unlock()

	if got := q.pickNext(); got != nil {
		t.Fatalf("picked %q while it was still in backoff", got.ID)
	}

	q.mu.Lock()
	q.tasks["waiting"].NextAttemptAt = time.Now().Add(-time.Second).UnixMilli()
	q.mu.Unlock()

	got := q.pickNext()
	if got == nil || got.ID != "waiting" {
		t.Fatal("a task whose backoff has elapsed should be runnable")
	}
	if got.NextAttemptAt != 0 {
		t.Error("dispatching should clear the backoff deadline")
	}
}

func TestManualRetryAcceptsSkippedTasks(t *testing.T) {
	m := newTestManager(t)
	m.Add(&model.Task{ID: "s1", WindowID: "win-1", Status: model.StatusSkipped})
	m.Retry("win-1", "s1")

	tasks := m.Tasks("win-1")
	if len(tasks) != 1 || tasks[0].Status != model.StatusPending {
		t.Fatalf("skipped task did not return to pending: %+v", tasks)
	}
}

func TestResolveLocalHonoursConflictPolicy(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got, err := resolveLocal(existing, model.ConflictOverwrite); err != nil || got != existing {
		t.Errorf("overwrite = %q, %v; want the original path", got, err)
	}
	if _, err := resolveLocal(existing, model.ConflictSkip); !errors.Is(err, errSkipped) {
		t.Errorf("skip returned %v, want errSkipped", err)
	}
	got, err := resolveLocal(existing, model.ConflictRename)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "report (1).pdf") {
		t.Errorf("rename = %q, want %q", got, filepath.Join(dir, "report (1).pdf"))
	}

	fresh := filepath.Join(dir, "new.pdf")
	for _, policy := range []model.ConflictPolicy{model.ConflictOverwrite, model.ConflictSkip, model.ConflictRename} {
		if got, err := resolveLocal(fresh, policy); err != nil || got != fresh {
			t.Errorf("%s on a free path = %q, %v; want it unchanged", policy, got, err)
		}
	}
}

func TestFreeLocalPathTreatsDotfilesAsNames(t *testing.T) {
	dir := t.TempDir()
	dotfile := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(dotfile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := freeLocalPath(dotfile, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, ".gitignore (1)") {
		t.Errorf("freeLocalPath = %q, want %q", got, filepath.Join(dir, ".gitignore (1)"))
	}
}
