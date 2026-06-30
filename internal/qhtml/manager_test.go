package qhtml

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
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
	if got.Percent != 100 {
		t.Fatalf("standalone core surface should be complete, got %d", got.Percent)
	}
	if len(got.ValueProposition) == 0 || !strings.Contains(got.ValueProposition[0], "full HTML scans") {
		t.Fatalf("value proposition must expose fullscan reduction: %#v", got.ValueProposition)
	}
	if !hasProductItem(got.Implemented, "precision_targeting_surface") {
		t.Fatalf("precision targeting surface missing: %#v", got.Implemented)
	}
	if !hasProductItem(got.Implemented, "media_slot_resolver") {
		t.Fatalf("media slot resolver missing: %#v", got.Implemented)
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

func TestRenderFolderWritesEscapedHTMLAndReceipt(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	if err := os.WriteFile(filepath.Join(laneRoot, "02", "r0", "xss.txt"), []byte("<script>alert(1)</script>"), 0o600); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(laneRoot, "dist", "qhtml.html")
	got, err := RenderFolder(RenderFolderRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		ExportPath:    exportPath,
		Title:         "QHTML Test",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 6, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "rendered_folder" || got.ExportDigest == "" || got.RenderDigest == "" || got.ReceiptPath == "" {
		t.Fatalf("unexpected render receipt: %#v", got)
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(data)
	if strings.Contains(rendered, "<script>alert(1)</script>") || !strings.Contains(rendered, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("rendered html did not escape lane content: %s", rendered)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	second, err := Manage(ManageRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ObservedAt:  time.Date(2026, 6, 30, 6, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.LaneDigest != got.LaneDigest {
		t.Fatalf("dist export self-contaminated lane digest: render=%s manage=%s", got.LaneDigest, second.LaneDigest)
	}
}

func TestRenderFolderRejectsExportInsideLaneOutsideDist(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	_, err := RenderFolder(RenderFolderRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ExportPath:  filepath.Join(laneRoot, "render.html"),
		ObservedAt:  time.Date(2026, 6, 30, 6, 2, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "dist/") {
		t.Fatalf("unsafe lane export should be rejected, got %v", err)
	}
}

func TestResolveMediaWritesReceiptAndCopiesExport(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	assetPath := filepath.Join(laneRoot, "04", "hero", "image.png")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(assetPath, []byte{0x89, 'P', 'N', 'G', 1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laneRoot, "04", "hero", "notes.txt"), []byte("not media"), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(laneRoot, "dist", "media")
	got, err := ResolveMedia(MediaRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		OutDir:        outDir,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 7, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "media_resolved" || got.AssetCount != 1 || got.MediaDigest == "" || got.ReceiptPath == "" {
		t.Fatalf("unexpected media receipt: %#v", got)
	}
	if got.Assets[0].SlotPath != "04/hero" || got.Assets[0].AssetPath != "04/hero/image.png" || got.Assets[0].MimeHint != "image/png" {
		t.Fatalf("unexpected media asset: %#v", got.Assets[0])
	}
	if _, err := os.Stat(filepath.Join(outDir, "04", "hero", "image.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestResolveMediaRejectsUnsafeAndOversizeInputs(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	assetPath := filepath.Join(laneRoot, "04", "hero.png")
	if err := os.WriteFile(assetPath, []byte{1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveMedia(MediaRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		SlotRoot:    "../04",
		ObservedAt:  time.Date(2026, 6, 30, 7, 1, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "inside lane root") {
		t.Fatalf("escaping slot root should be rejected, got %v", err)
	}
	_, err = ResolveMedia(MediaRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		OutDir:      filepath.Join(laneRoot, "media-export"),
		ObservedAt:  time.Date(2026, 6, 30, 7, 2, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "dist/") {
		t.Fatalf("unsafe media out-dir should be rejected, got %v", err)
	}
	_, err = ResolveMedia(MediaRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		MaxBytes:    2,
		ObservedAt:  time.Date(2026, 6, 30, 7, 3, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds max bytes") {
		t.Fatalf("oversize media should be rejected, got %v", err)
	}
}

func TestChunkMediaWritesStreamingChunkReceipt(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	assetPath := filepath.Join(laneRoot, "04", "hero", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o750); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 9000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(assetPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ChunkMedia(ChunkMediaRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		ChunkBytes:    4096,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "chunk_media" || got.AssetCount != 1 || got.ChunkDigest == "" || got.ReceiptPath == "" {
		t.Fatalf("unexpected chunk media receipt: %#v", got)
	}
	if got.Assets[0].ChunkCount != 3 || len(got.Assets[0].Chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %#v", got.Assets[0])
	}
	if got.Assets[0].Chunks[0].Offset != 0 || got.Assets[0].Chunks[1].Offset != 4096 || got.Assets[0].Chunks[2].Offset != 8192 {
		t.Fatalf("unexpected chunk offsets: %#v", got.Assets[0].Chunks)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestChunkMediaRejectsTooSmallChunk(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	_, err := ChunkMedia(ChunkMediaRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ChunkBytes:  1024,
		ObservedAt:  time.Date(2026, 6, 30, 10, 1, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "at least 4096") {
		t.Fatalf("too-small chunk should be rejected, got %v", err)
	}
}

func TestAdapterConformanceWritesReceipt(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	got, err := AdapterConformance(AdapterConformanceRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "adapter_conformant" || got.FailCount != 0 || got.PassCount == 0 || got.MatrixDigest == "" || got.ReceiptPath == "" {
		t.Fatalf("unexpected adapter conformance receipt: %#v", got)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterConformanceRejectsNonportableLane(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	if err := os.WriteFile(filepath.Join(laneRoot, "CON.txt"), []byte("reserved"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := AdapterConformance(AdapterConformanceRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ObservedAt:  time.Date(2026, 6, 30, 8, 1, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "adapter conformance failed") {
		t.Fatalf("nonportable lane should fail, got receipt=%#v err=%v", got, err)
	}
	if got.Status != "adapter_nonconformant" || got.FailCount == 0 {
		t.Fatalf("expected nonconformant receipt, got %#v", got)
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

func TestImportProposalWritesReceiptWithoutMutatingLane(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	targetPath := filepath.Join(laneRoot, "02", "r0", "c0.txt")
	before, err := digestFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(projectRoot, "export.html")
	if err := os.WriteFile(exportPath, []byte("<main>Edited export content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ImportProposal(ImportProposalRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		ExportPath:    exportPath,
		TargetPath:    "02/r0/c0.txt",
		Kind:          "cell",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "import_proposal" || got.ExportDigest == "" || got.LaneDigest == "" || got.TargetDigest == "" || got.ProposalDigest == "" {
		t.Fatalf("unexpected import proposal: %#v", got)
	}
	if got.PatchAction != "proposal_only" {
		t.Fatalf("import proposal must not mutate lane directly: %#v", got)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	after, err := digestFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("import proposal mutated lane target: %s != %s", before, after)
	}
}

func TestImportProposalRejectsBlankExportAndEscapingTarget(t *testing.T) {
	projectRoot, laneRoot, _ := setupManagedProject(t)
	blankPath := filepath.Join(projectRoot, "blank.html")
	if err := os.WriteFile(blankPath, []byte("<html><script>1</script></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ImportProposal(ImportProposalRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ExportPath:  blankPath,
		ObservedAt:  time.Date(2026, 6, 30, 3, 1, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "blank export") {
		t.Fatalf("blank export should be rejected, got %v", err)
	}
	exportPath := filepath.Join(projectRoot, "export.html")
	if err := os.WriteFile(exportPath, []byte("<main>Edited export content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ImportProposal(ImportProposalRequest{
		ProjectRoot: projectRoot,
		LaneRoot:    laneRoot,
		ExportPath:  exportPath,
		TargetPath:  "../source.html",
		ObservedAt:  time.Date(2026, 6, 30, 3, 2, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "inside lane root") {
		t.Fatalf("escaping import target should be rejected, got %v", err)
	}
}

func TestSealCombinesWitnessAndImportProposalReceipts(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	exportPath := filepath.Join(projectRoot, "export.html")
	reportPath := filepath.Join(projectRoot, "runner-report.json")
	if err := os.WriteFile(exportPath, []byte("<main>Sealed export content</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte(`{"runner":"playwright","ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	witness, err := Witness(WitnessRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		ExportPath:    exportPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 4, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := ImportProposal(ImportProposalRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		ExportPath:    exportPath,
		TargetPath:    "02/r0/c0.txt",
		Kind:          "cell",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 4, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := RunnerProof(RunnerProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-local",
		RunnerVersion: "1.0.0",
		Signature:     "signature-claim-1234567890",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 4, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	seal, err := Seal(SealRequest{
		ProjectRoot:        projectRoot,
		WitnessPath:        witness.WitnessPath,
		ImportProposalPath: proposal.ReceiptPath,
		RunnerProofPath:    proof.ReceiptPath,
		WriteEvidence:      true,
		ObservedAt:         time.Date(2026, 6, 30, 4, 3, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seal.Status != "vorq_seal" || !seal.PromotionReady || seal.SealDigest == "" {
		t.Fatalf("unexpected seal receipt: %#v", seal)
	}
	if seal.InputDigests["witness"] == "" || seal.InputDigests["import_proposal"] == "" || seal.InputDigests["runner_proof"] == "" {
		t.Fatalf("seal did not bind input digests: %#v", seal)
	}
	if _, err := os.Stat(seal.ReceiptPath); err != nil {
		t.Fatal(err)
	}
}

func TestSealRejectsMissingWitnessAndWrongSchema(t *testing.T) {
	projectRoot := t.TempDir()
	if _, err := Seal(SealRequest{ProjectRoot: projectRoot}); err == nil || !strings.Contains(err.Error(), "--witness required") {
		t.Fatalf("missing witness should be rejected, got %v", err)
	}
	badPath := filepath.Join(projectRoot, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"schema_version":"qhtml.import_proposal.v1","status":"import_proposal","product_id":"qhtml"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Seal(SealRequest{
		ProjectRoot: projectRoot,
		WitnessPath: badPath,
		ObservedAt:  time.Date(2026, 6, 30, 4, 3, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "wrong receipt schema") {
		t.Fatalf("wrong witness schema should be rejected, got %v", err)
	}
}

func TestRunnerProofWritesReceiptAndRejectsShortSignature(t *testing.T) {
	projectRoot := t.TempDir()
	reportPath := filepath.Join(projectRoot, "runner-report.json")
	if err := os.WriteFile(reportPath, []byte(`{"runner":"playwright","ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := RunnerProof(RunnerProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-local",
		RunnerVersion: "1.0.0",
		Signature:     "signature-claim-1234567890",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 4, 4, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "runner_proof" || got.ReportDigest == "" || got.ProofDigest == "" {
		t.Fatalf("unexpected runner proof: %#v", got)
	}
	if _, err := os.Stat(got.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	_, err = RunnerProof(RunnerProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-local",
		RunnerVersion: "1.0.0",
		Signature:     "short",
		ObservedAt:    time.Date(2026, 6, 30, 4, 5, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "at least 16") {
		t.Fatalf("short signature should be rejected, got %v", err)
	}
}

func TestVerifyRunnerProofWritesReceiptAndSealBindsVerification(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	exportPath := filepath.Join(projectRoot, "export.html")
	reportPath := filepath.Join(projectRoot, "runner-report.json")
	if err := os.WriteFile(exportPath, []byte("<main>Verified runner export</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte(`{"runner":"playwright","ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	reportDigest, err := digestFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	payload := runnerProofSignaturePayload("playwright-local", "1.0.0", reportDigest)
	signature := ed25519.Sign(privateKey, []byte(payload))
	proof, err := RunnerProof(RunnerProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-local",
		RunnerVersion: "1.0.0",
		Signature:     base64.StdEncoding.EncodeToString(signature),
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 5, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyRunnerProof(VerifyRunnerProofRequest{
		ProjectRoot:   projectRoot,
		ProofPath:     proof.ReceiptPath,
		PublicKey:     base64.StdEncoding.EncodeToString(publicKey),
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 5, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.Status != "runner_proof_verified" || !verified.Verified || verified.PublicKeyDigest == "" {
		t.Fatalf("unexpected runner proof verification: %#v", verified)
	}
	if _, err := os.Stat(verified.ReceiptPath); err != nil {
		t.Fatal(err)
	}
	witness, err := Witness(WitnessRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		ExportPath:    exportPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 5, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	seal, err := Seal(SealRequest{
		ProjectRoot:            projectRoot,
		WitnessPath:            witness.WitnessPath,
		RunnerProofPath:        proof.ReceiptPath,
		RunnerVerificationPath: verified.ReceiptPath,
		WriteEvidence:          true,
		ObservedAt:             time.Date(2026, 6, 30, 5, 3, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seal.InputDigests["runner_verification"] == "" {
		t.Fatalf("seal did not bind runner verification: %#v", seal)
	}
}

func TestVerifyRunnerProofRejectsBadSignature(t *testing.T) {
	projectRoot := t.TempDir()
	reportPath := filepath.Join(projectRoot, "runner-report.json")
	if err := os.WriteFile(reportPath, []byte(`{"runner":"playwright","ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	reportDigest, err := digestFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	payload := runnerProofSignaturePayload("playwright-local", "1.0.0", reportDigest)
	signature := ed25519.Sign(privateKey, []byte(payload))
	proof, err := RunnerProof(RunnerProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-local",
		RunnerVersion: "1.0.0",
		Signature:     base64.StdEncoding.EncodeToString(signature),
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 5, 4, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = VerifyRunnerProof(VerifyRunnerProofRequest{
		ProjectRoot: projectRoot,
		ProofPath:   proof.ReceiptPath,
		PublicKey:   base64.StdEncoding.EncodeToString(publicKey),
		ObservedAt:  time.Date(2026, 6, 30, 5, 5, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("bad signature should be rejected, got %v", err)
	}
}

func TestOPFSProofWritesReceiptAndSealBindsIt(t *testing.T) {
	projectRoot, laneRoot, sourcePath := setupManagedProject(t)
	exportPath := filepath.Join(projectRoot, "export.html")
	if err := os.WriteFile(exportPath, []byte("<main>OPFS export</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(projectRoot, "opfs-report.json")
	report := `{"opfs_available":true,"quota_bytes":1048576,"file_handle":true,"write_read_delete":true,"path_roundtrip":true,"relative_paths":true,"console_errors":0,"browser":"chromium","origin":"http://localhost"}`
	if err := os.WriteFile(reportPath, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	proof, err := OPFSProof(OPFSProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-opfs",
		RunnerVersion: "1.0.0",
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.Status != "opfs_proof" || proof.ProofDigest == "" || proof.ReportDigest == "" || proof.ReceiptPath == "" {
		t.Fatalf("unexpected OPFS proof: %#v", proof)
	}
	witness, err := Witness(WitnessRequest{
		ProjectRoot:   projectRoot,
		LaneRoot:      laneRoot,
		SourcePath:    sourcePath,
		ExportPath:    exportPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 9, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	seal, err := Seal(SealRequest{
		ProjectRoot:   projectRoot,
		WitnessPath:   witness.WitnessPath,
		OPFSProofPath: proof.ReceiptPath,
		WriteEvidence: true,
		ObservedAt:    time.Date(2026, 6, 30, 9, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seal.InputDigests["opfs_proof"] == "" {
		t.Fatalf("seal did not bind OPFS proof: %#v", seal)
	}
}

func TestOPFSProofRejectsBadReport(t *testing.T) {
	projectRoot := t.TempDir()
	reportPath := filepath.Join(projectRoot, "opfs-report.json")
	if err := os.WriteFile(reportPath, []byte(`{"opfs_available":false,"quota_bytes":0,"file_handle":false,"write_read_delete":false,"path_roundtrip":false,"relative_paths":false,"console_errors":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OPFSProof(OPFSProofRequest{
		ProjectRoot:   projectRoot,
		ReportPath:    reportPath,
		RunnerID:      "playwright-opfs",
		RunnerVersion: "1.0.0",
		ObservedAt:    time.Date(2026, 6, 30, 9, 3, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable OPFS") {
		t.Fatalf("bad OPFS report should be rejected, got %v", err)
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
