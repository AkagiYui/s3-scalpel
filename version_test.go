package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var semverish = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// TestVersionIsSemver guards the embedded VERSION file, which feeds both the
// About panel and the packaged bundle metadata.
func TestVersionIsSemver(t *testing.T) {
	if !semverish.MatchString(version) {
		t.Fatalf("VERSION = %q, want a semantic version like 1.2.3", version)
	}
}

// TestVersionMatchesBuildConfig keeps build/config.yml — which drives the
// generated bundle metadata (Info.plist, the Windows manifest, nfpm) — in step
// with the VERSION file that the binary itself reports.
func TestVersionMatchesBuildConfig(t *testing.T) {
	data, err := os.ReadFile("build/config.yml")
	if err != nil {
		t.Fatalf("read build/config.yml: %v", err)
	}
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "version:") || strings.HasPrefix(trimmed, "version: '3'") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		if idx := strings.Index(value, "#"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		want = strings.Trim(value, `"'`)
		break
	}
	if want == "" {
		t.Fatal("no application version found in build/config.yml")
	}
	if want != version {
		t.Fatalf("build/config.yml version = %q but VERSION = %q; bump both together", want, version)
	}
}

// TestBuildVersionPresent catches an empty or missing COMMIT file, which would
// leave the About panel without any build identity.
func TestBuildVersionPresent(t *testing.T) {
	if buildVersion == "" {
		t.Fatal("COMMIT is empty; it must contain at least \"dev\"")
	}
}
