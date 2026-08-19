package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// progressInterval bounds how often a task emits a progress event.
const progressInterval = 200 * time.Millisecond

// errSkipped is returned when the conflict policy decided the destination should
// be left alone. It is a normal outcome, not a failure.
var errSkipped = errors.New("destination exists; skipped")

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
	// Parallel transfers call this from several goroutines at once, so the
	// throttle clock is an atomic rather than a captured local.
	var lastEmit atomic.Int64
	onProgress := func(transferred int64) {
		q.mu.Lock()
		t.Transferred = transferred
		size := t.Size
		q.mu.Unlock()

		now := time.Now().UnixNano()
		prev := lastEmit.Load()
		if now-prev < int64(progressInterval) || !lastEmit.CompareAndSwap(prev, now) {
			return
		}
		q.mgr.deps.Emit("task:progress", map[string]any{
			"windowId":    q.windowID,
			"id":          t.ID,
			"transferred": transferred,
			"size":        size,
		})
	}

	switch t.Type {
	case model.TaskUpload:
		return q.upload(ctx, cl, t, s, onProgress)
	case model.TaskDownload:
		return q.download(ctx, cl, t, s, onProgress)
	case model.TaskDelete:
		if strings.HasSuffix(t.Key, "/") {
			return s3x.DeletePrefix(ctx, cl, t.Bucket, t.Key)
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	case model.TaskCopy:
		return q.copy(ctx, cl, t, s, onProgress)
	case model.TaskMove:
		if err := q.copy(ctx, cl, t, s, onProgress); err != nil {
			return err
		}
		return s3x.DeleteObject(ctx, cl, t.Bucket, t.Key, "")
	default:
		return fmt.Errorf("unknown task type %q", t.Type)
	}
}

// upload sends a local file, honouring the conflict policy and resuming a
// multipart upload that a previous attempt left open.
func (q *windowQueue) upload(ctx context.Context, cl *s3.Client, t *model.Task, s model.AppSettings, onProgress s3x.ProgressFunc) error {
	key, err := resolveRemote(ctx, cl, t.Bucket, t.Key, s.ConflictPolicy)
	if err != nil {
		return err
	}
	q.mu.Lock()
	t.ResolvedKey = key
	resume := t.UploadID
	q.mu.Unlock()

	opts := s3x.UploadOptions{
		StorageClass:    s.UploadStorageClass,
		SSEAlgorithm:    s.UploadSSE,
		KMSKeyID:        s.UploadKMSKeyID,
		PartConcurrency: s.PartConcurrency,
		ResumeUploadID:  resume,
		OnUploadID: func(id string) {
			// Persisting the id is what lets the next attempt resume, and what
			// lets a final failure abort the upload instead of orphaning it.
			q.mu.Lock()
			t.UploadID = id
			q.mu.Unlock()
			q.mgr.scheduleSave()
		},
	}
	return s3x.Upload(ctx, cl, t.Bucket, key, t.LocalPath, s.MultipartEnabled, s.PartSize, opts, onProgress)
}

// download fetches an object, honouring the conflict policy for the local file.
func (q *windowQueue) download(ctx context.Context, cl *s3.Client, t *model.Task, s model.AppSettings, onProgress s3x.ProgressFunc) error {
	path, err := resolveLocal(t.LocalPath, s.ConflictPolicy)
	if err != nil {
		return err
	}
	q.mu.Lock()
	t.ResolvedPath = path
	q.mu.Unlock()

	return s3x.Download(ctx, cl, t.Bucket, t.Key, path, s3x.DownloadOptions{
		PartSize:        s.PartSize,
		PartConcurrency: s.PartConcurrency,
		Multipart:       s.MultipartEnabled,
	}, onProgress)
}

// copy performs a copy for a task, using a server-side CopyObject when source and
// destination share a connection, and a streaming cross-connection copy when the
// destination is a different connection (different account/endpoint).
func (q *windowQueue) copy(ctx context.Context, srcClient *s3.Client, t *model.Task, s model.AppSettings, onProgress s3x.ProgressFunc) error {
	destClient := srcClient
	if t.DestConnID != "" && t.DestConnID != t.ConnectionID {
		destConn, ok := q.mgr.deps.GetConnection(t.DestConnID)
		if !ok {
			return fmt.Errorf("destination connection not found")
		}
		var err error
		destClient, err = q.mgr.deps.GetClient(ctx, destConn)
		if err != nil {
			return err
		}
	}

	destKey, err := resolveRemote(ctx, destClient, t.DestBucket, t.DestKey, s.ConflictPolicy)
	if err != nil {
		return err
	}
	q.mu.Lock()
	t.ResolvedKey = destKey
	q.mu.Unlock()

	if destClient == srcClient {
		return s3x.CopyObject(ctx, srcClient, t.Bucket, t.Key, t.DestBucket, destKey)
	}
	return s3x.StreamCopy(ctx, srcClient, destClient, t.Bucket, t.Key, t.DestBucket, destKey,
		s.PartSize, s3x.UploadOptions{
			StorageClass: s.UploadStorageClass,
			SSEAlgorithm: s.UploadSSE,
			KMSKeyID:     s.UploadKMSKeyID,
		}, onProgress)
}

// resolveRemote applies the conflict policy to a destination object key,
// returning the key to actually write (or errSkipped).
func resolveRemote(ctx context.Context, cl *s3.Client, bucket, key string, policy model.ConflictPolicy) (string, error) {
	switch policy {
	case model.ConflictSkip:
		exists, err := s3x.ObjectExists(ctx, cl, bucket, key)
		if err != nil {
			return "", err
		}
		if exists {
			return "", errSkipped
		}
		return key, nil
	case model.ConflictRename:
		return s3x.FreeKey(ctx, cl, bucket, key, 0)
	default:
		return key, nil
	}
}

// resolveLocal applies the conflict policy to a download destination path.
func resolveLocal(path string, policy model.ConflictPolicy) (string, error) {
	switch policy {
	case model.ConflictSkip:
		_, err := os.Stat(path)
		switch {
		case err == nil:
			return "", errSkipped
		case os.IsNotExist(err):
			return path, nil
		default:
			return "", err
		}
	case model.ConflictRename:
		return freeLocalPath(path, 100)
	default:
		return path, nil
	}
}

// freeLocalPath returns path when it is unused, otherwise the first free
// "name (n).ext" sibling.
func freeLocalPath(path string, limit int) (string, error) {
	switch _, err := os.Stat(path); {
	case os.IsNotExist(err):
		return path, nil
	case err != nil:
		return "", err
	}
	dir, base := filepath.Dir(path), filepath.Base(path)
	stem, ext := base, ""
	// A leading dot belongs to the name (".gitignore"), not to an extension.
	if idx := strings.LastIndex(base, "."); idx > 0 {
		stem, ext = base[:idx], base[idx:]
	}
	for n := 1; n <= limit; n++ {
		candidate := filepath.Join(dir, stem+" ("+strconv.Itoa(n)+")"+ext)
		switch _, err := os.Stat(candidate); {
		case os.IsNotExist(err):
			return candidate, nil
		case err != nil:
			return "", err
		}
	}
	return "", fmt.Errorf("no free name found for %s", path)
}
