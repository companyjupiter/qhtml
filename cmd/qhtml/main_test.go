package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunStatus(t *testing.T) {
	if code := run([]string{"status"}); code != 0 {
		t.Fatalf("status failed with code %d", code)
	}
}

func TestRunRefresh(t *testing.T) {
	projectRoot := t.TempDir()
	laneRoot := filepath.Join(projectRoot, "lane")
	if err := os.MkdirAll(filepath.Join(laneRoot, "02", "r0"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "02", "r0", "c0.txt"), []byte("title"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"refresh", "--project", projectRoot, "--lane-root", laneRoot, "--write"}); code != 0 {
		t.Fatalf("refresh failed with code %d", code)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".qhtml", "managed")); err != nil {
		t.Fatal(err)
	}
}
