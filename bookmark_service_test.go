package main

import (
	"testing"

	"s3scalpel/internal/model"
	"s3scalpel/internal/store"
)

func newBookmarkService(t *testing.T) *BookmarkService {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &BookmarkService{core: &Core{store: st}}
}

func TestBookmarkAddAndList(t *testing.T) {
	s := newBookmarkService(t)

	saved, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "photos", Prefix: "2024/holiday"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.CreatedAt == 0 {
		t.Errorf("a saved bookmark needs an id and a timestamp: %+v", saved)
	}
	// Prefixes are stored in the one shape the browser navigates with.
	if saved.Prefix != "2024/holiday/" {
		t.Errorf("prefix = %q, want %q", saved.Prefix, "2024/holiday/")
	}
	if saved.Label != "photos/2024/holiday" {
		t.Errorf("default label = %q, want %q", saved.Label, "photos/2024/holiday")
	}

	if got := s.List(); len(got) != 1 || got[0].ID != saved.ID {
		t.Errorf("List returned %+v", got)
	}
}

func TestBookmarkLabelsABucketRoot(t *testing.T) {
	s := newBookmarkService(t)
	saved, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "photos"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Label != "photos" {
		t.Errorf("default label for a bucket root = %q, want %q", saved.Label, "photos")
	}
}

func TestBookmarkAddIsIdempotentForTheSameLocation(t *testing.T) {
	s := newBookmarkService(t)
	first, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "b", Prefix: "x/", Label: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	// The same place typed without its trailing slash is the same place.
	second, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "b", Prefix: "x", Label: "New"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Error("re-bookmarking a location should update it, not duplicate it")
	}
	if second.CreatedAt != first.CreatedAt {
		t.Error("the original creation time should be kept")
	}
	list := s.List()
	if len(list) != 1 || list[0].Label != "New" {
		t.Errorf("list = %+v, want a single bookmark relabelled to New", list)
	}
}

func TestBookmarkRejectsIncompleteLocations(t *testing.T) {
	s := newBookmarkService(t)
	for _, b := range []model.Bookmark{
		{Bucket: "b"},
		{ConnectionID: "c1"},
	} {
		if _, err := s.Add(b); err == nil {
			t.Errorf("Add(%+v) should have been rejected", b)
		}
	}
	if got := s.List(); len(got) != 0 {
		t.Errorf("nothing should have been stored, got %+v", got)
	}
}

func TestBookmarkDelete(t *testing.T) {
	s := newBookmarkService(t)
	a, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(model.Bookmark{ConnectionID: "c1", Bucket: "b"}); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	list := s.List()
	if len(list) != 1 || list[0].Bucket != "b" {
		t.Errorf("after delete, list = %+v", list)
	}

	// Deleting something that is not there is not an error.
	if err := s.Delete("missing"); err != nil {
		t.Errorf("deleting an unknown id returned %v", err)
	}
}

func TestBookmarksSurviveAReload(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	core := &Core{store: st}
	svc := &BookmarkService{core: core}
	if _, err := svc.Add(model.Bookmark{ConnectionID: "c1", Bucket: "b", Prefix: "deep/"}); err != nil {
		t.Fatal(err)
	}

	// A fresh Core over the same directory loads what was written.
	st2, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := &Core{store: st2}
	reloaded.loadBookmarks()
	list := (&BookmarkService{core: reloaded}).List()
	if len(list) != 1 || list[0].Prefix != "deep/" {
		t.Errorf("reloaded bookmarks = %+v", list)
	}
}

func TestNormalizeBookmarkPrefix(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"   ":      "",
		"a":        "a/",
		"a/":       "a/",
		"/a/b":     "a/b/",
		"  a/b/  ": "a/b/",
		"a/b/c/":   "a/b/c/",
	}
	for in, want := range cases {
		if got := normalizeBookmarkPrefix(in); got != want {
			t.Errorf("normalizeBookmarkPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
