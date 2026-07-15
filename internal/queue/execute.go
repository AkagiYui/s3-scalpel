package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

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
			StorageClass:    s.UploadStorageClass,
			SSEAlgorithm:    s.UploadSSE,
			KMSKeyID:        s.UploadKMSKeyID,
			PartConcurrency: s.PartConcurrency,
		}
		return s3x.Upload(ctx, cl, t.Bucket, t.Key, t.LocalPath, s.MultipartEnabled, s.PartSize, opts, onProgress)
	case model.TaskDownload:
		dopts := s3x.DownloadOptions{
			PartSize:        s.PartSize,
			PartConcurrency: s.PartConcurrency,
			Multipart:       s.MultipartEnabled,
		}
		return s3x.Download(ctx, cl, t.Bucket, t.Key, t.LocalPath, dopts, onProgress)
	case model.TaskDelete:
		if strings.HasSuffix(t.Key, "/") {
			return s3x.DeletePrefix(ctx, cl, t.Bucket, t.Key)
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	case model.TaskCopy:
		return q.copy(ctx, cl, t, onProgress)
	case model.TaskMove:
		if err := q.copy(ctx, cl, t, onProgress); err != nil {
			return err
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	default:
		return fmt.Errorf("unknown task type %q", t.Type)
	}
}

// copy performs a copy for a task, using a server-side CopyObject when source and
// destination share a connection, and a streaming cross-connection copy when the
// destination is a different connection (different account/endpoint).
func (q *windowQueue) copy(ctx context.Context, srcClient *s3.Client, t *model.Task, onProgress s3x.ProgressFunc) error {
	if t.DestConnID == "" || t.DestConnID == t.ConnectionID {
		return s3x.CopyObject(ctx, srcClient, t.Bucket, t.Key, t.DestBucket, t.DestKey)
	}
	destConn, ok := q.mgr.deps.GetConnection(t.DestConnID)
	if !ok {
		return fmt.Errorf("destination connection not found")
	}
	destClient, err := q.mgr.deps.GetClient(ctx, destConn)
	if err != nil {
		return err
	}
	return s3x.StreamCopy(ctx, srcClient, destClient, t.Bucket, t.Key, t.DestBucket, t.DestKey, s3x.UploadOptions{}, onProgress)
}
