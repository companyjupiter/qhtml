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
	if len(got.ValueProposition) == 0 || !strings.Contains(got.ValueProposition[0], "full HTML scans") {
		t.Fatalf("value proposition must expose fullscan reduction: %#v", got.ValueProposition)
	}
	if !hasProductItem(got.Implemented, "precision_targeting_surface") {
		t.Fatalf("precision targeting surface missing: %#v", got.Implemented)
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

func TestWitnessWritesRenderReceipt(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	exportPath := filepath.Join(projectRoot, "export.html")
	if err := os.WriteFile(exportPath, []byte("<main>rendered</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Witness(WitnessRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		ExportPath:    exportPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "render_witness" || got.RenderInputDigest == "" || got.ExportDigest == "" {
		t.Fatalf("unexpected witness: %#v", got)
	}
	if _, err := os.Stat(got.WitnessPath); err != nil {
		t.Fatal(err)
	}
}

func TestWitnessRejectsMissingExport(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	_, err := Witness(WitnessRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SourcePath:  sourcePath,
		ObservedAt:  time.Date(2026, 6, 30, 1, 1, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("missing export should be rejected")
	}
}

func TestVisualWitnessWritesReceipt(t *testing.T) {
	projectRoot := t.TempDir()
	exportPath := filepath.Join(projectRoot, "export.html")
	consolePath := filepath.Join(projectRoot, "console.json")
	screenshotPath := filepath.Join(projectRoot, "screen.png")
	if err := os.WriteFile(exportPath, []byte("<main>Visible content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consolePath, []byte(`[{"level":"info","text":"ok"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(screenshotPath, []byte{0x89, 'P', 'N', 'G', 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := VisualWitness(VisualWitnessRequest{
		ProjectRoot:       projectRoot,
		ExportPath:        exportPath,
		ConsoleReportPath: consolePath,
		ScreenshotPath:    screenshotPath,
		Viewport:          "desktop",
		WriteEvidence:     true,
		ObservedAt:        time.Date(2026, 6, 30, 1, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "visual_witness" || !got.NonBlank || got.ConsoleErrors != 0 || got.ScreenshotBytes == 0 {
		t.Fatalf("unexpected visual witness: %#v", got)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestVisualWitnessRejectsBlankExportAndConsoleErrors(t *testing.T) {
	projectRoot := t.TempDir()
	blankPath := filepath.Join(projectRoot, "blank.html")
	if err := os.WriteFile(blankPath, []byte("<html><script>1</script></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VisualWitness(VisualWitnessRequest{ProjectRoot: projectRoot, ExportPath: blankPath}); err == nil {
		t.Fatalf("blank export should be rejected")
	}
	exportPath := filepath.Join(projectRoot, "export.html")
	consolePath := filepath.Join(projectRoot, "console.json")
	if err := os.WriteFile(exportPath, []byte("<main>Visible content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(consolePath, []byte(`[{"level":"error","text":"boom"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VisualWitness(VisualWitnessRequest{ProjectRoot: projectRoot, ExportPath: exportPath, ConsoleReportPath: consolePath}); err == nil {
		t.Fatalf("console errors should be rejected")
	}
}

func TestLayoutWitnessWritesReceipt(t *testing.T) {
	projectRoot := t.TempDir()
	exportPath := filepath.Join(projectRoot, "export.html")
	reportPath := filepath.Join(projectRoot, "layout.json")
	if err := os.WriteFile(exportPath, []byte("<main>Visible content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := `{"viewports":[{"name":"desktop","width":1440,"height":900,"nonblank":true,"console_errors":0,"overflow_x":0,"overflow_y":0},{"name":"mobile","width":390,"height":844,"nonblank":true,"console_errors":0,"overflow_x":0,"overflow_y":0}]}`
	if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LayoutWitness(LayoutWitnessRequest{
		ProjectRoot:   projectRoot,
		ExportPath:    exportPath,
		ReportPath:    reportPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 1, 3, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "layout_witness" || got.ViewportCount != 2 || got.LayoutDigest == "" {
		t.Fatalf("unexpected layout witness: %#v", got)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutWitnessRejectsBadViewportReports(t *testing.T) {
	projectRoot := t.TempDir()
	exportPath := filepath.Join(projectRoot, "export.html")
	if err := os.WriteFile(exportPath, []byte("<main>Visible content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"blank":    `{"viewports":[{"name":"mobile","width":390,"height":844,"nonblank":false,"console_errors":0,"overflow_x":0,"overflow_y":0}]}`,
		"console":  `{"viewports":[{"name":"desktop","width":1440,"height":900,"nonblank":true,"console_errors":1,"overflow_x":0,"overflow_y":0}]}`,
		"overflow": `{"viewports":[{"name":"desktop","width":1440,"height":900,"nonblank":true,"console_errors":0,"overflow_x":1,"overflow_y":0}]}`,
		"invalid":  `{"viewports":[{"name":"desktop","width":0,"height":900,"nonblank":true,"console_errors":0,"overflow_x":0,"overflow_y":0}]}`,
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			reportPath := filepath.Join(projectRoot, name+".json")
			if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LayoutWitness(LayoutWitnessRequest{
				ProjectRoot: projectRoot,
				ExportPath:  exportPath,
				ReportPath:  reportPath,
			})
			if err == nil {
				t.Fatalf("bad layout report %s should be rejected", name)
			}
		})
	}
}

func TestTargetTombstoneAndRollbackWriteReceipts(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	targetPath := "02/r0/c0.txt"
	target, err := Target(TargetRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		TargetPath:    targetPath,
		Kind:          "cell",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 2, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Status != "target" || target.TargetPath != targetPath || target.Digest == "" {
		t.Fatalf("unexpected target receipt: %#v", target)
	}
	if _, err := os.Stat(target.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	tombstone, err := Tombstone(TombstoneRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		TargetPath:    targetPath,
		Kind:          "cell",
		Reason:        "duplicate cell",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 2, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if tombstone.Status != "tombstone_proposal" || tombstone.TargetDigest != target.Digest {
		t.Fatalf("unexpected tombstone receipt: %#v", tombstone)
	}
	if _, err := os.Stat(tombstone.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	rollback, err := Rollback(RollbackRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		TargetPath:    targetPath,
		Kind:          "cell",
		ToDigest:      target.Digest,
		SourceReceipt: tombstone.ReceiptPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 2, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Status != "rollback_proposal" || rollback.CurrentDigest != target.Digest || rollback.ToDigest != target.Digest {
		t.Fatalf("unexpected rollback receipt: %#v", rollback)
	}
	if _, err := os.Stat(rollback.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestTargetRejectsEscapingLaneRoot(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	_, err := Target(TargetRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		TargetPath:  "../source.html",
		ObservedAt:  time.Date(2026, 6, 30, 2, 3, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "inside lane root") {
		t.Fatalf("escaping target should be rejected, got %v", err)
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

func hasProductItem(items []ProductItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
