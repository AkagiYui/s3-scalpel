// Package queue implements per-window operation queues with bounded concurrency,
// priority scheduling, retry, progress reporting and disk persistence.
package queue

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
	"s3scalpel/internal/store"
)

// Deps are the collaborators the queue needs, injected to avoid import cycles.
type Deps struct {
	GetConnection func(id string) (model.Connection, bool)
	GetClient     func(ctx context.Context, c model.Connection) (*s3.Client, error)
	Emit          func(event string, data any)
	Settings      func() model.AppSettings
	Notify        func(title, body string, isError bool)
}

const tasksFile = "tasks.json"

// Manager owns every window's queue and the shared task store.
type Manager struct {
	mu     sync.Mutex
	queues map[string]*windowQueue
	deps   Deps
	store  *store.Store

	saveTimer *time.Timer
}

// NewManager loads persisted tasks, marks previously-running tasks as failed
// (unclean shutdown recovery) and returns a ready manager.
func NewManager(st *store.Store, deps Deps) *Manager {
	m := &Manager{queues: map[string]*windowQueue{}, deps: deps, store: st}

	var persisted []*model.Task
	_, _ = st.ReadJSON(tasksFile, &persisted)
	for _, t := range persisted {
		if t.Status == model.StatusRunning {
			t.Status = model.StatusFailed
			if t.Error == "" {
				t.Error = "interrupted by shutdown"
			}
		}
		q := m.queueFor(t.WindowID)
		q.tasks[t.ID] = t
		q.order = append(q.order, t.ID)
	}
	return m
}

// queueFor returns (creating if needed) the queue for a window id. Caller need
// not hold the lock; this method takes it.
func (m *Manager) queueFor(windowID string) *windowQueue {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q, ok := m.queues[windowID]; ok {
		return q
	}
	s := m.deps.Settings()
	q := &windowQueue{
		windowID:    windowID,
		tasks:       map[string]*model.Task{},
		active:      map[string]context.CancelFunc{},
		concurrency: max(1, s.Concurrency),
		autoConsume: s.AutoConsumeQueue,
		mgr:         m,
		wake:        make(chan struct{}, 1),
	}
	m.queues[windowID] = q
	go q.scheduler()
	return q
}

// EnsureWindow registers a window and adopts any orphaned tasks (tasks whose
// originating window no longer exists across restarts) into the first window.
func (m *Manager) EnsureWindow(windowID string, activeWindows []string) *windowQueue {
	q := m.queueFor(windowID)

	// Adopt orphans: move tasks belonging to windows not in activeWindows into
	// this (the requesting) window so the user can still see/retry them.
	active := map[string]bool{windowID: true}
	for _, w := range activeWindows {
		active[w] = true
	}
	m.mu.Lock()
	var adopted []*model.Task
	for wid, oq := range m.queues {
		if active[wid] || wid == windowID {
			continue
		}
		oq.mu.Lock()
		for _, id := range oq.order {
			t := oq.tasks[id]
			if t == nil {
				continue
			}
			t.WindowID = windowID
			adopted = append(adopted, t)
		}
		oq.tasks = map[string]*model.Task{}
		oq.order = nil
		oq.mu.Unlock()
		delete(m.queues, wid)
	}
	m.mu.Unlock()

	if len(adopted) > 0 {
		q.mu.Lock()
		for _, t := range adopted {
			q.tasks[t.ID] = t
			q.order = append(q.order, t.ID)
		}
		q.mu.Unlock()
		q.signal()
		m.scheduleSave()
		m.emitTasks(windowID)
	}
	return q
}

// Queue returns the queue for a window, creating it if needed.
func (m *Manager) Queue(windowID string) *windowQueue { return m.queueFor(windowID) }

// Add appends a task to its window's queue and wakes the scheduler.
func (m *Manager) Add(t *model.Task) {
	q := m.queueFor(t.WindowID)
	q.mu.Lock()
	q.tasks[t.ID] = t
	q.order = append(q.order, t.ID)
	q.mu.Unlock()
	q.signal()
	m.scheduleSave()
	m.emitTasks(t.WindowID)
}

// snapshot returns a copy of a window's tasks sorted by creation order.
func (m *Manager) snapshot(windowID string) []model.Task {
	q := m.queueFor(windowID)
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]model.Task, 0, len(q.order))
	for _, id := range q.order {
		if t := q.tasks[id]; t != nil {
			out = append(out, *t)
		}
	}
	return out
}

func (m *Manager) emitTasks(windowID string) {
	if m.deps.Emit == nil {
		return
	}
	m.deps.Emit("tasks:changed", map[string]any{
		"windowId": windowID,
		"tasks":    m.snapshot(windowID),
	})
}

// scheduleSave debounces persistence of all tasks across windows.
func (m *Manager) scheduleSave() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	m.saveTimer = time.AfterFunc(400*time.Millisecond, m.saveNow)
}

// saveNow writes every task to disk immediately.
func (m *Manager) saveNow() {
	m.mu.Lock()
	var all []*model.Task
	for _, q := range m.queues {
		q.mu.Lock()
		for _, id := range q.order {
			if t := q.tasks[id]; t != nil {
				cp := *t
				all = append(all, &cp)
			}
		}
		q.mu.Unlock()
	}
	m.mu.Unlock()
	_ = m.store.WriteJSON(tasksFile, all)
}

// Flush persists immediately (call on shutdown).
func (m *Manager) Flush() { m.saveNow() }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// windowQueue is a single window's queue with its own scheduler.
type windowQueue struct {
	windowID    string
	mu          sync.Mutex
	tasks       map[string]*model.Task
	order       []string
	concurrency int
	autoConsume bool
	drain       bool // manual "start" requested while autoConsume is off
	active      map[string]context.CancelFunc
	mgr         *Manager
	wake        chan struct{}
}

func (q *windowQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// scheduler dispatches runnable tasks whenever woken.
func (q *windowQueue) scheduler() {
	for range q.wake {
		for {
			t := q.pickNext()
			if t == nil {
				break
			}
			q.run(t)
		}
	}
}

// pickNext selects the highest-priority pending task if dispatch is allowed and
// a concurrency slot is free; it marks the task running under the lock.
func (q *windowQueue) pickNext() *model.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.active) >= q.concurrency {
		return nil
	}
	if !q.autoConsume && !q.drain {
		return nil
	}
	now := time.Now().UnixMilli()
	var best *model.Task
	for _, id := range q.order {
		t := q.tasks[id]
		if t == nil || t.Status != model.StatusPending {
			continue
		}
		// A task waiting out its retry backoff stays pending but is not runnable.
		if t.NextAttemptAt > now {
			continue
		}
		if best == nil || t.Priority > best.Priority {
			best = t
		}
	}
	if best == nil {
		// nothing runnable: clear a one-shot drain once everything settles.
		if q.drain && len(q.active) == 0 && !q.hasPendingLocked() {
			q.drain = false
		}
		return nil
	}
	best.Status = model.StatusRunning
	best.Error = ""
	best.NextAttemptAt = 0
	best.UpdatedAt = now
	return best
}

// hasPendingLocked reports whether any task is still waiting to run, including
// ones counting down a retry backoff. Callers must hold q.mu.
func (q *windowQueue) hasPendingLocked() bool {
	for _, t := range q.tasks {
		if t != nil && t.Status == model.StatusPending {
			return true
		}
	}
	return false
}

// classify maps an execution result onto a terminal task status. Cancellation is
// matched with errors.Is because the AWS SDK wraps it in *smithy.OperationError,
// so a plain equality check would misreport cancelled work as failed.
func classify(err error) model.TaskStatus {
	switch {
	case err == nil:
		return model.StatusCompleted
	case errors.Is(err, errSkipped):
		return model.StatusSkipped
	case errors.Is(err, context.Canceled):
		return model.StatusCanceled
	default:
		return model.StatusFailed
	}
}

// backoff returns the delay before retry attempt n (1-based): 1s, 2s, 4s, ...
// capped at 30s.
func backoff(attempt int) time.Duration {
	const (
		base = time.Second
		max  = 30 * time.Second
	)
	d := base << min(attempt-1, 16)
	if d > max || d <= 0 {
		return max
	}
	return d
}

// abortDanglingUpload discards the multipart upload a task was holding open.
// Called when a task reaches a terminal state other than success.
func (q *windowQueue) abortDanglingUpload(t *model.Task) {
	if t.UploadID == "" || t.Type != model.TaskUpload {
		return
	}
	key := t.ResolvedKey
	if key == "" {
		key = t.Key
	}
	conn, ok := q.mgr.deps.GetConnection(t.ConnectionID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cl, err := q.mgr.deps.GetClient(ctx, conn)
	if err != nil {
		return
	}
	_ = s3x.AbortUpload(ctx, cl, t.Bucket, key, t.UploadID)

	q.mu.Lock()
	if live := q.tasks[t.ID]; live != nil {
		live.UploadID = ""
	}
	q.mu.Unlock()
}

// run executes a task in its own goroutine.
func (q *windowQueue) run(t *model.Task) {
	ctx, cancel := context.WithCancel(context.Background())
	q.mu.Lock()
	q.active[t.ID] = cancel
	q.mu.Unlock()
	q.mgr.emitTasks(q.windowID)
	q.mgr.scheduleSave()

	go func() {
		err := q.execute(ctx, t)
		q.mu.Lock()
		delete(q.active, t.ID)
		t.UpdatedAt = time.Now().UnixMilli()
		status := classify(err)
		maxRetries := q.mgr.deps.Settings().MaxAutoRetries

		// A transient failure goes back to pending with an exponential backoff
		// rather than landing in front of the user as a permanent error.
		retrying := status == model.StatusFailed &&
			t.Retries < maxRetries &&
			s3x.IsRetriable(err)

		switch {
		case retrying:
			t.Status = model.StatusPending
			t.Retries++
			t.Transferred = 0
			t.Error = err.Error()
			t.NextAttemptAt = time.Now().Add(backoff(t.Retries)).UnixMilli()
		case status == model.StatusCompleted:
			t.Status = status
			t.Transferred = t.Size
			t.Error = ""
			t.UploadID = ""
		case status == model.StatusSkipped:
			t.Status = status
			t.Error = ""
			t.UploadID = ""
		case status == model.StatusCanceled:
			t.Status = status
			t.Error = ""
		default:
			t.Status = status
			t.Error = err.Error()
		}
		// Snapshot the finished task so everything below reads a stable copy
		// without holding the queue lock.
		done := *t
		q.mu.Unlock()

		if retrying {
			// Wake the scheduler once the backoff elapses; nothing else will.
			time.AfterFunc(time.Until(time.UnixMilli(done.NextAttemptAt)), q.signal)
		} else {
			// Giving up (or being cancelled) means the multipart upload this
			// task was holding open must be discarded, or its parts stay
			// billable forever.
			q.abortDanglingUpload(&done)
			q.mgr.notify(done.Status, string(done.Type), done.Key)
		}
		q.mgr.emitTasks(q.windowID)
		q.mgr.scheduleSave()
		if done.Status == model.StatusCompleted {
			q.mgr.deps.Emit("operation:done", map[string]any{
				"windowId":     q.windowID,
				"type":         string(done.Type),
				"connectionId": done.ConnectionID,
				"bucket":       done.Bucket,
				"key":          done.Key,
				"destBucket":   done.DestBucket,
				"destKey":      done.DestKey,
			})
		}
		q.signal()
	}()
}

func (m *Manager) notify(status model.TaskStatus, ttype, key string) {
	if m.deps.Notify == nil {
		return
	}
	// core.Notify gates on the user's notification settings.
	switch status {
	case model.StatusCompleted:
		m.deps.Notify("Operation complete", ttype+": "+key, false)
	case model.StatusFailed:
		m.deps.Notify("Operation failed", ttype+": "+key, true)
	}
}

// Control operations -------------------------------------------------------

// SetConcurrency updates a window queue's parallelism.
func (m *Manager) SetConcurrency(windowID string, n int) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	q.concurrency = max(1, n)
	q.mu.Unlock()
	q.signal()
}

// ApplySettings pushes changed global defaults (queue parallelism and automatic
// dispatch) onto every live window queue, so the Settings page and the per-window
// task-panel controls stay in agreement.
func (m *Manager) ApplySettings(concurrency int, autoConsume bool) {
	m.mu.Lock()
	queues := make([]*windowQueue, 0, len(m.queues))
	for _, q := range m.queues {
		queues = append(queues, q)
	}
	m.mu.Unlock()

	for _, q := range queues {
		q.mu.Lock()
		q.concurrency = max(1, concurrency)
		q.autoConsume = autoConsume
		q.mu.Unlock()
		q.signal()
		m.emitState(q.windowID)
	}
}

// SetAutoConsume toggles automatic dispatch for a window.
func (m *Manager) SetAutoConsume(windowID string, on bool) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	q.autoConsume = on
	q.mu.Unlock()
	q.signal()
	m.emitState(windowID)
}

// Start requests a one-shot drain of pending tasks (used when autoConsume off).
func (m *Manager) Start(windowID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	q.drain = true
	q.mu.Unlock()
	q.signal()
}

// Retry resets a failed/canceled task to pending.
func (m *Manager) Retry(windowID, taskID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	if t := q.tasks[taskID]; t != nil && isRetryable(t.Status) {
		t.Status = model.StatusPending
		t.Error = ""
		t.Transferred = 0
		t.NextAttemptAt = 0
		t.Retries++
		t.UpdatedAt = time.Now().UnixMilli()
	}
	q.mu.Unlock()
	q.signal()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// RetryAllFailed re-queues every failed task in a window.
func (m *Manager) RetryAllFailed(windowID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	for _, id := range q.order {
		t := q.tasks[id]
		if t != nil && isRetryable(t.Status) {
			t.Status = model.StatusPending
			t.Error = ""
			t.Transferred = 0
			t.NextAttemptAt = 0
			t.Retries++
		}
	}
	q.mu.Unlock()
	q.signal()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// isRetryable reports whether a task in this state can be re-queued by hand.
func isRetryable(status model.TaskStatus) bool {
	switch status {
	case model.StatusFailed, model.StatusCanceled, model.StatusSkipped:
		return true
	default:
		return false
	}
}

// Cancel stops a running task or removes a pending one.
func (m *Manager) Cancel(windowID, taskID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	if cancel, ok := q.active[taskID]; ok {
		cancel()
	} else if t := q.tasks[taskID]; t != nil && t.Status == model.StatusPending {
		t.Status = model.StatusCanceled
	}
	q.mu.Unlock()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// SetPriority changes a pending task's priority.
func (m *Manager) SetPriority(windowID, taskID string, priority int) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	if t := q.tasks[taskID]; t != nil {
		t.Priority = priority
	}
	q.mu.Unlock()
	q.signal()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// Remove deletes a finished task from the list.
func (m *Manager) Remove(windowID, taskID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	if t := q.tasks[taskID]; t != nil && t.Status != model.StatusRunning {
		delete(q.tasks, taskID)
		q.order = removeStr(q.order, taskID)
	}
	q.mu.Unlock()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// ClearFinished removes all completed/failed/canceled tasks from a window.
func (m *Manager) ClearFinished(windowID string) {
	q := m.queueFor(windowID)
	q.mu.Lock()
	var keep []string
	for _, id := range q.order {
		t := q.tasks[id]
		if t == nil {
			continue
		}
		if t.Status == model.StatusRunning || t.Status == model.StatusPending {
			keep = append(keep, id)
		} else {
			delete(q.tasks, id)
		}
	}
	q.order = keep
	q.mu.Unlock()
	m.emitTasks(windowID)
	m.scheduleSave()
}

// Tasks returns a snapshot for a window.
func (m *Manager) Tasks(windowID string) []model.Task { return m.snapshot(windowID) }

// State returns the queue control flags for a window.
func (m *Manager) State(windowID string) map[string]any {
	q := m.queueFor(windowID)
	q.mu.Lock()
	defer q.mu.Unlock()
	return map[string]any{
		"autoConsume": q.autoConsume,
		"concurrency": q.concurrency,
	}
}

func (m *Manager) emitState(windowID string) {
	if m.deps.Emit != nil {
		m.deps.Emit("queue:state", map[string]any{"windowId": windowID, "state": m.State(windowID)})
	}
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
