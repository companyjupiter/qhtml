package qhtml

import (
	"os"
	"path/filepath"
	"strings"
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

func TestManageStateInsideLaneDoesNotSelfContaminateDigest(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	stateRoot := filepath.Join(laneRoot, ".qhtml", "managed")
	first, err := Manage(ManageRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		StateRoot:     stateRoot,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 29, 2, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		StateRoot:   stateRoot,
		ObservedAt:  time.Date(2026, 6, 29, 2, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "current" || second.NeedsRenderRefresh {
		t.Fatalf("managed state inside lane self-contaminated digest: first=%#v second=%#v", first, second)
	}
	if first.LaneDigest != second.LaneDigest {
		t.Fatalf("lane digest drifted from managed state write: %s != %s", first.LaneDigest, second.LaneDigest)
	}
}

func TestManageDeletionChangesLaneDigest(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	first, err := Manage(ManageRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 29, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(laneRoot, "02", "r0", "c0.txt")); err != nil {
		t.Fatal(err)
	}
	deleted, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 3, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Status != "changed" || !deleted.LaneChanged || !deleted.NeedsRenderRefresh {
		t.Fatalf("deletion was not detected: first=%#v deleted=%#v", first, deleted)
	}
}

func TestManageSymlinkHashesTargetWithoutFollowing(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	outside := filepath.Join(projectRoot, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(laneRoot, "02", "r0", "linked.txt")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink unavailable on this platform or permission profile: %v", err)
	}
	first, err := Manage(ManageRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SymlinkCount != 1 {
		t.Fatalf("expected one symlink, got %#v", first)
	}
	if err := os.WriteFile(outside, []byte("outside-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 4, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.LaneDigest != first.LaneDigest || second.NeedsRenderRefresh {
		t.Fatalf("symlink target contents should not affect lane digest: first=%#v second=%#v", first, second)
	}
	if err := os.Remove(linkPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(projectRoot, "other.txt"), linkPath); err != nil {
		t.Fatal(err)
	}
	changed, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 29, 4, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed.LaneChanged || !changed.NeedsRenderRefresh {
		t.Fatalf("symlink target path change should affect digest: %#v", changed)
	}
}

func TestManageLockBlocksConcurrentWriter(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	stateRoot := filepath.Join(projectRoot, ".qhtml", "managed")
	lockPath := managedLockPath(managedRoot(projectRoot, stateRoot), laneRoot, sourcePath)
	release, err := acquireLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, err = Manage(ManageRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		StateRoot:     stateRoot,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 29, 5, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "lock already held") {
		t.Fatalf("expected lock error, got %v", err)
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
