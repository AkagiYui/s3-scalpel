package main

import (
	"testing"
	"time"
)

func TestParentPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"folder/", ""},
		{"a/b/folder/", "a/b/"},
		{"a/folder/", "a/"},
		{"x/", ""},
	}
	for _, c := range cases {
		if got := parentPrefix(c.in); got != c.want {
			t.Errorf("parentPrefix(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestRandIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := randID()
		if id == "" {
			t.Fatal("empty id")
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestPlainKeysDropsFolders(t *testing.T) {
	got := plainKeys([]string{"a.txt", "dir/", "b/c.txt", "b/"})
	want := []string{"a.txt", "b/c.txt"}
	if len(got) != len(want) {
		t.Fatalf("plainKeys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plainKeys = %v, want %v", got, want)
		}
	}
	if got := plainKeys(nil); len(got) != 0 {
		t.Errorf("plainKeys(nil) = %v, want empty", got)
	}
}

func TestOpRegistryCancelsAndReleases(t *testing.T) {
	r := newOpRegistry()

	ctx, done := r.begin("op-1", time.Minute)
	if r.count() != 1 {
		t.Fatalf("registry holds %d operations, want 1", r.count())
	}
	if !r.cancel("op-1") {
		t.Fatal("cancel should report that it found the operation")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelling the id did not cancel its context")
	}
	if r.cancel("op-1") {
		t.Error("cancelling twice should report that nothing was running")
	}
	done()
	if r.count() != 0 {
		t.Errorf("registry leaked %d operations", r.count())
	}
}

func TestOpRegistryReleaseRemovesEntry(t *testing.T) {
	r := newOpRegistry()
	_, done := r.begin("op-1", time.Minute)
	done()
	if r.count() != 0 {
		t.Fatalf("release left %d operations registered", r.count())
	}
	if r.cancel("op-1") {
		t.Error("a released operation should no longer be cancellable")
	}
}

func TestOpRegistrySupersedesReusedID(t *testing.T) {
	r := newOpRegistry()
	first, done1 := r.begin("op-1", time.Minute)
	defer done1()
	second, done2 := r.begin("op-1", time.Minute)
	defer done2()

	select {
	case <-first.Done():
	case <-time.After(time.Second):
		t.Fatal("re-using an id should cancel the operation it replaces")
	}
	if second.Err() != nil {
		t.Error("the replacement operation should still be live")
	}
	if r.count() != 1 {
		t.Errorf("registry holds %d operations, want 1", r.count())
	}
}

func TestOpRegistryWithoutIDStillGivesAContext(t *testing.T) {
	r := newOpRegistry()
	ctx, done := r.begin("", time.Minute)
	defer done()
	if ctx.Err() != nil {
		t.Error("an anonymous operation should start live")
	}
	if r.count() != 0 {
		t.Error("an anonymous operation must not be registered")
	}
}
