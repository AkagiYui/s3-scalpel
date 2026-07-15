package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// execute performs the actual S3 work for a task, reporting throttled progress.
func (q *windowQueue) execute(ctx context.Context, t *model.Task) error {
	conn, ok := q.mgr.deps.GetConnection(t.ConnectionID)
	if !ok {
		return fmt.Errorf("connection not found")
	}
	cl, err := q.mgr.deps.GetClient(ctx, conn)
	if err != nil {
		return err
	}
	s := q.mgr.deps.Settings()

	// Throttle progress events to ~5/sec per task to avoid flooding the bridge.
	var lastEmit time.Time
	onProgress := func(transferred int64) {
		q.mu.Lock()
		t.Transferred = transferred
		q.mu.Unlock()
		now := time.Now()
		if now.Sub(lastEmit) > 200*time.Millisecond {
			lastEmit = now
			q.mgr.deps.Emit("task:progress", map[string]any{
				"windowId":    q.windowID,
				"id":          t.ID,
				"transferred": transferred,
				"size":        t.Size,
			})
		}
	}

	switch t.Type {
	case model.TaskUpload:
		opts := s3x.UploadOptions{
			StorageClass: s.UploadStorageClass,
			SSEAlgorithm: s.UploadSSE,
			KMSKeyID:     s.UploadKMSKeyID,
		}
		return s3x.Upload(ctx, cl, t.Bucket, t.Key, t.LocalPath, s.MultipartEnabled, s.PartSize, opts, onProgress)
	case model.TaskDownload:
		return s3x.Download(ctx, cl, t.Bucket, t.Key, t.LocalPath, onProgress)
	case model.TaskDelete:
		if strings.HasSuffix(t.Key, "/") {
			return s3x.DeletePrefix(ctx, cl, t.Bucket, t.Key)
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	case model.TaskCopy:
		return s3x.CopyObject(ctx, cl, t.Bucket, t.Key, t.DestBucket, t.DestKey)
	case model.TaskMove:
		if err := s3x.CopyObject(ctx, cl, t.Bucket, t.Key, t.DestBucket, t.DestKey); err != nil {
			return err
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	default:
		return fmt.Errorf("unknown task type %q", t.Type)
	}
}
