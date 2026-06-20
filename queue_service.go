package main

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// QueueService exposes the per-window operation queue to the frontend. Every
// method takes the caller's windowID so each window has an independent queue.
type QueueService struct{ core *Core }

// RegisterWindow ensures the window's queue exists (adopting orphaned tasks from
// closed windows) and returns the current tasks.
func (s *QueueService) RegisterWindow(windowID string) []model.Task {
	active := s.core.activeWindowIDs()
	s.core.queue.EnsureWindow(windowID, active)
	return s.core.queue.Tasks(windowID)
}

// Tasks returns a snapshot of a window's tasks.
func (s *QueueService) Tasks(windowID string) []model.Task { return s.core.queue.Tasks(windowID) }

// State returns the queue control flags for a window.
func (s *QueueService) State(windowID string) map[string]any { return s.core.queue.State(windowID) }

func now() int64 { return time.Now().UnixMilli() }

func (s *QueueService) add(t *model.Task) {
	t.ID = randID()
	t.Status = model.StatusPending
	t.CreatedAt = now()
	t.UpdatedAt = now()
	s.core.queue.Add(t)
}

// EnqueueUpload queues uploads for files and (recursively) directories. The base
// name of each path is preserved under destPrefix.
func (s *QueueService) EnqueueUpload(windowID, connID, bucket, destPrefix string, localPaths []string, priority int) (int, error) {
	count := 0
	for _, p := range localPaths {
		info, err := os.Stat(p)
		if err != nil {
			return count, err
		}
		base := filepath.Base(p)
		if info.IsDir() {
			err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(p, path)
				rel = filepath.ToSlash(rel)
				key := s3x.JoinKey(destPrefix, base+"/"+rel)
				fi, _ := d.Info()
				var size int64
				if fi != nil {
					size = fi.Size()
				}
				s.add(&model.Task{
					WindowID: windowID, Type: model.TaskUpload, ConnectionID: connID,
					Bucket: bucket, Key: key, LocalPath: path, Size: size, Priority: priority,
				})
				count++
				return nil
			})
			if err != nil {
				return count, err
			}
		} else {
			key := s3x.JoinKey(destPrefix, base)
			s.add(&model.Task{
				WindowID: windowID, Type: model.TaskUpload, ConnectionID: connID,
				Bucket: bucket, Key: key, LocalPath: p, Size: info.Size(), Priority: priority,
			})
			count++
		}
	}
	return count, nil
}

// EnqueueDownload queues downloads for keys into destDir, expanding folder keys
// (ending in "/") recursively and recreating their structure under destDir.
func (s *QueueService) EnqueueDownload(windowID, connID, bucket string, keys []string, destDir string, priority int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			parent := parentPrefix(key)
			entries, err := s3x.ListAllObjects(ctx, cl, bucket, key)
			if err != nil {
				return count, err
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Key, "/") {
					continue
				}
				rel := strings.TrimPrefix(e.Key, parent)
				local := filepath.Join(destDir, filepath.FromSlash(rel))
				s.add(&model.Task{
					WindowID: windowID, Type: model.TaskDownload, ConnectionID: connID,
					Bucket: bucket, Key: e.Key, LocalPath: local, Size: e.Size, Priority: priority,
				})
				count++
			}
		} else {
			local := filepath.Join(destDir, path.Base(key))
			s.add(&model.Task{
				WindowID: windowID, Type: model.TaskDownload, ConnectionID: connID,
				Bucket: bucket, Key: key, LocalPath: local, Priority: priority,
			})
			count++
		}
	}
	return count, nil
}

// EnqueueDownloadAs queues a single-file download to an explicit local path.
func (s *QueueService) EnqueueDownloadAs(windowID, connID, bucket, key, destPath string, priority int) error {
	s.add(&model.Task{
		WindowID: windowID, Type: model.TaskDownload, ConnectionID: connID,
		Bucket: bucket, Key: key, LocalPath: destPath, Priority: priority,
	})
	return nil
}

// EnqueueDelete queues deletion of the given keys (folder keys delete the whole
// prefix during execution).
func (s *QueueService) EnqueueDelete(windowID, connID, bucket string, keys []string, priority int) (int, error) {
	for _, key := range keys {
		s.add(&model.Task{
			WindowID: windowID, Type: model.TaskDelete, ConnectionID: connID,
			Bucket: bucket, Key: key, Priority: priority,
		})
	}
	return len(keys), nil
}

// EnqueueCopy queues copy or move operations into destBucket/destPrefix,
// expanding folder keys recursively.
func (s *QueueService) EnqueueCopy(windowID, connID, srcBucket string, keys []string, destBucket, destPrefix string, move bool, priority int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return 0, err
	}
	ttype := model.TaskCopy
	if move {
		ttype = model.TaskMove
	}
	count := 0
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			parent := parentPrefix(key)
			entries, err := s3x.ListAllObjects(ctx, cl, srcBucket, key)
			if err != nil {
				return count, err
			}
			for _, e := range entries {
				rel := strings.TrimPrefix(e.Key, parent)
				destKey := s3x.JoinKey(destPrefix, rel)
				s.add(&model.Task{
					WindowID: windowID, Type: ttype, ConnectionID: connID,
					Bucket: srcBucket, Key: e.Key, DestBucket: destBucket, DestKey: destKey,
					Size: e.Size, Priority: priority,
				})
				count++
			}
		} else {
			destKey := s3x.JoinKey(destPrefix, path.Base(key))
			s.add(&model.Task{
				WindowID: windowID, Type: ttype, ConnectionID: connID,
				Bucket: srcBucket, Key: key, DestBucket: destBucket, DestKey: destKey, Priority: priority,
			})
			count++
		}
	}
	return count, nil
}

// Control passthroughs -----------------------------------------------------

func (s *QueueService) Start(windowID string) { s.core.queue.Start(windowID) }
func (s *QueueService) SetAutoConsume(windowID string, on bool) {
	s.core.queue.SetAutoConsume(windowID, on)
}
func (s *QueueService) SetConcurrency(windowID string, n int) {
	s.core.queue.SetConcurrency(windowID, n)
}
func (s *QueueService) Retry(windowID, taskID string)  { s.core.queue.Retry(windowID, taskID) }
func (s *QueueService) RetryAllFailed(windowID string) { s.core.queue.RetryAllFailed(windowID) }
func (s *QueueService) Cancel(windowID, taskID string) { s.core.queue.Cancel(windowID, taskID) }
func (s *QueueService) SetPriority(windowID, taskID string, p int) {
	s.core.queue.SetPriority(windowID, taskID, p)
}
func (s *QueueService) Remove(windowID, taskID string) { s.core.queue.Remove(windowID, taskID) }
func (s *QueueService) ClearFinished(windowID string)  { s.core.queue.ClearFinished(windowID) }

// parentPrefix returns everything up to and including the parent of a folder key
// (e.g. "a/b/folder/" -> "a/b/").
func parentPrefix(folderKey string) string {
	trimmed := strings.TrimSuffix(folderKey, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return ""
	}
	return trimmed[:idx+1]
}
