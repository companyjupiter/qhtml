package qhtml

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManageDetectsLaneAndSourceChange(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	first, err := Manage(ManageRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "initialized" || !first.LaneChanged || !first.SourceChanged || !first.NeedsRenderRefresh {
		t.Fatalf("first refresh should initialize: %#v", first)
	}
	if _, err := os.Stat(first.StatePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	second, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 1, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "current" || second.LaneChanged || second.SourceChanged || second.NeedsRenderRefresh {
		t.Fatalf("unchanged refresh should be current: %#v", second)
	}
	if err := os.WriteFile(sourcePath, []byte("<main>changed</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceChanged, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 1, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sourceChanged.Status != "changed" || !sourceChanged.SourceChanged || !sourceChanged.NeedsRenderRefresh {
		t.Fatalf("source change was not detected: %#v", sourceChanged)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "02", "r0", "c1.txt"), []byte("new cell"), 0o600); err != nil {
		t.Fatal(err)
	}
	laneChanged, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 1, 3, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if laneChanged.Status != "changed" || !laneChanged.LaneChanged || !laneChanged.NeedsRenderRefresh {
		t.Fatalf("lane change was not detected: %#v", laneChanged)
	}
}

func TestStatusSurface(t *testing.T) {
	got := Status(ProductStatusRequest{ObservedAt: time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC)})
	if got.SchemaVersion != ProductStatusSchemaVersion || got.ProductID != "qhtml" {
		t.Fatalf("unexpected status: %#v", got)
	}
	if got.Percent <= 0 || got.Percent >= 100 {
		t.Fatalf("standalone seed should be partial, got %d", got.Percent)
	}
}

func setupManagedProject(t *testing.T) (string, string, string) {
	t.Helper()
	projectRoot := t.TempDir()
	laneRoot := filepath.Join(projectRoot, "lane")
	sourcePath := filepath.Join(projectRoot, "source.html")
	for _, dir := range []string{
		filepath.Join(laneRoot, "00", "r0"),
		filepath.Join(laneRoot, "02", "r0"),
		filepath.Join(laneRoot, "04", "m0"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "00", "r0", "c0.txt"), []byte("digest"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "02", "r0", "c0.txt"), []byte("title"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("<main>original</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	return projectRoot, laneRoot, sourcePath
}
