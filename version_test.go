package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// tomlVersionRe matches the Version line of FyneApp.toml, e.g. Version = "0.2.0".
var tomlVersionRe = regexp.MustCompile(`(?m)^\s*Version\s*=\s*"([^"]+)"`)

// changelogHeadingRe matches a "## vX.Y.Z" heading and captures the version
// without the leading "v" and without any title after the dash.
var changelogHeadingRe = regexp.MustCompile(`(?m)^## v(\d+\.\d+\.\d+)(?:\s.*)?$`)

func TestVersionMatchesFyneApp(t *testing.T) {
	data, err := os.ReadFile("FyneApp.toml")
	if err != nil {
		t.Fatal(err)
	}
	m := tomlVersionRe.FindSubmatch(data)
	if m == nil {
		t.Fatal("no Version in FyneApp.toml")
	}
	if got := string(m[1]); got != Version {
		t.Errorf("FyneApp.toml Version = %q, Version const = %q", got, Version)
	}
}

func TestVersionMatchesChangelog(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	all := changelogHeadingRe.FindAllSubmatch(data, -1)
	if len(all) == 0 {
		t.Fatal("no ## vX.Y.Z heading in CHANGELOG.md")
	}
	last := all[len(all)-1]
	if got := string(last[1]); got != Version {
		t.Errorf("last CHANGELOG heading = %q, Version const = %q", strings.TrimSpace(string(last[0])), Version)
	}
}
