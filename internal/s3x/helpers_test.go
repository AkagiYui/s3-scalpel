package s3x

import "testing"

func TestJoinKey(t *testing.T) {
	cases := []struct{ prefix, name, want string }{
		{"", "a.txt", "a.txt"},
		{"dir/", "a.txt", "dir/a.txt"},
		{"dir", "a.txt", "dir/a.txt"},
		{"a/b/", "c.txt", "a/b/c.txt"},
	}
	for _, c := range cases {
		if got := JoinKey(c.prefix, c.name); got != c.want {
			t.Errorf("JoinKey(%q,%q)=%q want %q", c.prefix, c.name, got, c.want)
		}
	}
}

func TestFolderName(t *testing.T) {
	cases := []struct{ key, prefix, want string }{
		{"dir/sub/", "dir/", "sub"},
		{"a/", "", "a"},
		{"x/y/z/", "x/y/", "z"},
	}
	for _, c := range cases {
		if got := folderName(c.key, c.prefix); got != c.want {
			t.Errorf("folderName(%q,%q)=%q want %q", c.key, c.prefix, got, c.want)
		}
	}
}

func TestContentTypeForName(t *testing.T) {
	if got := contentTypeForName("photo.png"); got == "" || got == "application/octet-stream" {
		t.Errorf("png should map to an image type, got %q", got)
	}
	if got := contentTypeForName("blob.unknownext"); got != "application/octet-stream" {
		t.Errorf("unknown ext should fall back, got %q", got)
	}
}
