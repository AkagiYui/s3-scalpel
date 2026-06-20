package main

import "testing"

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
