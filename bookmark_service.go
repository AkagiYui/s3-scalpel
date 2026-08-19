package main

import (
	"fmt"
	"strings"
	"time"

	"s3scalpel/internal/model"
)

const bookmarksFile = "bookmarks.json"

// BookmarkService stores favourite locations — a bucket, optionally a prefix
// inside it — so a deeply nested working directory is one click away instead of
// a breadcrumb walk. The list is shared across windows and persisted.
type BookmarkService struct{ core *Core }

// List returns every bookmark, newest last.
func (s *BookmarkService) List() []model.Bookmark {
	s.core.bookmarkMu.RLock()
	defer s.core.bookmarkMu.RUnlock()
	out := make([]model.Bookmark, len(s.core.bookmarks))
	copy(out, s.core.bookmarks)
	return out
}

// Add saves a location. Re-adding somewhere already bookmarked updates its label
// rather than creating a duplicate entry.
func (s *BookmarkService) Add(b model.Bookmark) (model.Bookmark, error) {
	if b.ConnectionID == "" || b.Bucket == "" {
		return b, fmt.Errorf("a bookmark needs a connection and a bucket")
	}
	b.Prefix = normalizeBookmarkPrefix(b.Prefix)
	if strings.TrimSpace(b.Label) == "" {
		b.Label = defaultBookmarkLabel(b)
	}

	s.core.bookmarkMu.Lock()
	for i, existing := range s.core.bookmarks {
		if existing.ConnectionID == b.ConnectionID && existing.Bucket == b.Bucket && existing.Prefix == b.Prefix {
			b.ID, b.CreatedAt = existing.ID, existing.CreatedAt
			s.core.bookmarks[i] = b
			s.core.bookmarkMu.Unlock()
			return b, s.persist()
		}
	}
	b.ID = randID()
	b.CreatedAt = time.Now().UnixMilli()
	s.core.bookmarks = append(s.core.bookmarks, b)
	s.core.bookmarkMu.Unlock()
	return b, s.persist()
}

// Delete removes a bookmark by id.
func (s *BookmarkService) Delete(id string) error {
	s.core.bookmarkMu.Lock()
	kept := s.core.bookmarks[:0]
	for _, b := range s.core.bookmarks {
		if b.ID != id {
			kept = append(kept, b)
		}
	}
	s.core.bookmarks = kept
	s.core.bookmarkMu.Unlock()
	return s.persist()
}

func (s *BookmarkService) persist() error {
	s.core.bookmarkMu.RLock()
	list := make([]model.Bookmark, len(s.core.bookmarks))
	copy(list, s.core.bookmarks)
	s.core.bookmarkMu.RUnlock()

	if err := s.core.store.WriteJSON(bookmarksFile, list); err != nil {
		return err
	}
	s.core.emit("bookmarks:changed", nil)
	return nil
}

// normalizeBookmarkPrefix keeps stored prefixes in the one shape the browser
// uses: empty, or ending in a slash.
func normalizeBookmarkPrefix(prefix string) string {
	p := strings.TrimPrefix(strings.TrimSpace(prefix), "/")
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// defaultBookmarkLabel names an unlabelled bookmark after where it points.
func defaultBookmarkLabel(b model.Bookmark) string {
	if b.Prefix == "" {
		return b.Bucket
	}
	return b.Bucket + "/" + strings.TrimSuffix(b.Prefix, "/")
}
