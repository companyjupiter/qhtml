package qhtml

import (
	"errors"
	"fmt"
	htmlpkg "html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const RenderFolderSchemaVersion = "qhtml.render_folder.v1"

type RenderFolderRequest struct {
	ProjectRoot   string
	LaneRoot      string
	ExportPath    string
	Title         string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type RenderedItem struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Digest string `json:"digest,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
	Target string `json:"target,omitempty"`
}

type RenderFolderReceipt struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	ExportPath    string            `json:"export_path"`
	StateRoot     string            `json:"state_root"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	LaneDigest    string            `json:"lane_digest"`
	ExportDigest  string            `json:"export_digest,omitempty"`
	RenderDigest  string            `json:"render_digest"`
	FileCount     int               `json:"file_count"`
	DirCount      int               `json:"dir_count"`
	SymlinkCount  int               `json:"symlink_count"`
	RenderedItems []RenderedItem    `json:"rendered_items"`
	NegativeCases []string          `json:"negative_cases"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func RenderFolder(req RenderFolderRequest) (RenderFolderReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return RenderFolderReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return RenderFolderReceipt{}, errors.New("qhtml lane root required")
	}
	if strings.TrimSpace(req.ExportPath) == "" {
		return RenderFolderReceipt{}, errors.New("qhtml render-folder --out required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	exportPath := absPath(projectRoot, req.ExportPath)
	if err := validateRenderExportPath(laneRoot, exportPath); err != nil {
		return RenderFolderReceipt{}, err
	}
	stateRoot := renderRoot(projectRoot, req.StateRoot)
	laneDigest, fileCount, dirCount, symlinkCount, ignored, err := digestTree(laneRoot, []string{stateRoot, filepath.Join(laneRoot, "dist")})
	if err != nil {
		return RenderFolderReceipt{}, err
	}
	items, err := collectRenderItems(laneRoot)
	if err != nil {
		return RenderFolderReceipt{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "QHTML Render"
	}
	html := renderFolderHTML(title, laneRoot, laneDigest, items)
	exportDigest := sha256Hex([]byte(html))
	renderDigest := sha256Hex([]byte(strings.Join([]string{
		RenderFolderSchemaVersion,
		laneDigest,
		exportDigest,
		strings.Join(renderItemKeys(items), "\n"),
	}, "\n")))
	receiptPath := filepath.Join(stateRoot, renderDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_render_folder.json")
	receipt := RenderFolderReceipt{
		SchemaVersion: RenderFolderSchemaVersion,
		Status:        "rendered_folder",
		ProductID:     "qhtml",
		LaneRoot:      slashClean(laneRoot),
		ExportPath:    slashClean(exportPath),
		StateRoot:     slashClean(stateRoot),
		LaneDigest:    laneDigest,
		ExportDigest:  exportDigest,
		RenderDigest:  renderDigest,
		FileCount:     fileCount,
		DirCount:      dirCount,
		SymlinkCount:  symlinkCount,
		RenderedItems: items,
		NegativeCases: []string{
			"missing_lane_rejected",
			"missing_export_path_rejected",
			"export_inside_lane_source_rejected_except_dist",
			"html_content_escaped_before_projection",
		},
		Policy:     "folder_lane_is_source_truth; html_export_is_disposable_projection; dist_exports_do_not_self_contaminate_lane_digest",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"ignored":      strings.Join(ignored, ","),
			"renderer":     "qhtml_go_folder_projection_v1",
		},
	}
	if req.WriteEvidence {
		if err := writeTextFile(exportPath, html); err != nil {
			return receipt, err
		}
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func validateRenderExportPath(laneRoot, exportPath string) error {
	rel, err := filepath.Rel(laneRoot, exportPath)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return errors.New("qhtml export path cannot be lane root")
	}
	if rel != ".." && !strings.HasPrefix(rel, "../") {
		if rel == "dist" || strings.HasPrefix(rel, "dist/") {
			return nil
		}
		return errors.New("qhtml export inside lane root must be under dist/")
	}
	return nil
}

func collectRenderItems(laneRoot string) ([]RenderedItem, error) {
	var items []RenderedItem
	err := filepath.WalkDir(laneRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if shouldIgnorePath(laneRoot, path, d, nil) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(laneRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			items = append(items, RenderedItem{Path: rel, Kind: "dir"})
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			items = append(items, RenderedItem{Path: rel, Kind: "symlink", Target: target})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		items = append(items, RenderedItem{
			Path:   rel,
			Kind:   "file",
			Digest: sha256Hex(data),
			Bytes:  len(data),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func renderFolderHTML(title, laneRoot, laneDigest string, items []RenderedItem) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>" + htmlpkg.EscapeString(title) + "</title>\n")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;margin:32px;line-height:1.5;color:#111}main{max-width:960px;margin:auto}section{border:1px solid #ddd;border-radius:8px;margin:12px 0;padding:16px}pre{white-space:pre-wrap;overflow:auto;background:#f6f6f6;padding:12px;border-radius:6px}code{font-family:ui-monospace,Consolas,monospace}</style>\n")
	b.WriteString("</head>\n<body data-qhtml-render=\"folder\" data-qhtml-lane-digest=\"" + htmlpkg.EscapeString(laneDigest) + "\">\n")
	b.WriteString("<main>\n<h1>" + htmlpkg.EscapeString(title) + "</h1>\n")
	b.WriteString("<p><code data-qhtml-meta=\"lane-root\">" + htmlpkg.EscapeString(slashClean(laneRoot)) + "</code></p>\n")
	for _, item := range items {
		path := htmlpkg.EscapeString(item.Path)
		kind := htmlpkg.EscapeString(item.Kind)
		b.WriteString("<section class=\"qhtml-cell\" data-qhtml-path=\"" + path + "\" data-qhtml-kind=\"" + kind + "\">\n")
		b.WriteString("<h2><code>" + path + "</code></h2>\n")
		if item.Kind == "file" {
			b.WriteString("<p data-qhtml-digest=\"" + htmlpkg.EscapeString(item.Digest) + "\">" + fmt.Sprintf("%d bytes", item.Bytes) + "</p>\n")
			data, err := os.ReadFile(filepath.Join(laneRoot, filepath.FromSlash(item.Path)))
			if err == nil {
				b.WriteString("<pre>" + htmlpkg.EscapeString(string(data)) + "</pre>\n")
			}
		} else if item.Kind == "symlink" {
			b.WriteString("<p>symlink target: <code>" + htmlpkg.EscapeString(item.Target) + "</code></p>\n")
		} else {
			b.WriteString("<p>directory</p>\n")
		}
		b.WriteString("</section>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func renderItemKeys(items []RenderedItem) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, strings.Join([]string{item.Kind, item.Path, item.Digest, item.Target}, "\t"))
	}
	sort.Strings(keys)
	return keys
}

func renderRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "renders")
}

func writeTextFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
