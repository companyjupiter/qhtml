package qhtml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const VisualWitnessSchemaVersion = "qhtml.visual_witness.v1"

type VisualWitnessRequest struct {
	ProjectRoot       string
	ExportPath        string
	ConsoleReportPath string
	ScreenshotPath    string
	Viewport          string
	StateRoot         string
	WriteEvidence     bool
	ObservedAt        time.Time
}

type VisualWitnessReceipt struct {
	SchemaVersion     string            `json:"schema_version"`
	Status            string            `json:"status"`
	ProductID         string            `json:"product_id"`
	ProjectRoot       string            `json:"project_root"`
	ExportPath        string            `json:"export_path"`
	ConsoleReportPath string            `json:"console_report_path,omitempty"`
	ScreenshotPath    string            `json:"screenshot_path,omitempty"`
	StateRoot         string            `json:"state_root"`
	ReceiptPath       string            `json:"receipt_path,omitempty"`
	Viewport          string            `json:"viewport"`
	ExportDigest      string            `json:"export_digest"`
	ConsoleDigest     string            `json:"console_digest,omitempty"`
	ScreenshotDigest  string            `json:"screenshot_digest,omitempty"`
	VisualDigest      string            `json:"visual_digest"`
	NonBlank          bool              `json:"nonblank"`
	ConsoleErrors     int               `json:"console_errors"`
	ScreenshotBytes   int64             `json:"screenshot_bytes,omitempty"`
	Checks            []string          `json:"checks"`
	NegativeCases     []string          `json:"negative_cases"`
	Policy            string            `json:"policy"`
	ObservedAt        string            `json:"observed_at"`
	Details           map[string]string `json:"details,omitempty"`
}

func VisualWitness(req VisualWitnessRequest) (VisualWitnessReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return VisualWitnessReceipt{}, err
	}
	if strings.TrimSpace(req.ExportPath) == "" {
		return VisualWitnessReceipt{}, errors.New("qhtml export path required for visual witness")
	}
	exportPath := absPath(projectRoot, req.ExportPath)
	exportDigest, err := digestFile(exportPath)
	if err != nil {
		return VisualWitnessReceipt{}, err
	}
	exportData, err := os.ReadFile(exportPath)
	if err != nil {
		return VisualWitnessReceipt{}, err
	}
	nonBlank := htmlHasVisibleText(string(exportData))
	if !nonBlank {
		return VisualWitnessReceipt{}, errors.New("qhtml visual witness rejected blank export")
	}

	consolePath := ""
	consoleDigest := ""
	consoleErrors := 0
	if strings.TrimSpace(req.ConsoleReportPath) != "" {
		consolePath = absPath(projectRoot, req.ConsoleReportPath)
		consoleDigest, err = digestFile(consolePath)
		if err != nil {
			return VisualWitnessReceipt{}, err
		}
		consoleErrors, err = countConsoleErrors(consolePath)
		if err != nil {
			return VisualWitnessReceipt{}, err
		}
		if consoleErrors > 0 {
			return VisualWitnessReceipt{}, errors.New("qhtml visual witness rejected console errors")
		}
	}

	screenshotPath := ""
	screenshotDigest := ""
	var screenshotBytes int64
	if strings.TrimSpace(req.ScreenshotPath) != "" {
		screenshotPath = absPath(projectRoot, req.ScreenshotPath)
		screenshotDigest, err = digestFile(screenshotPath)
		if err != nil {
			return VisualWitnessReceipt{}, err
		}
		info, statErr := os.Stat(screenshotPath)
		if statErr != nil {
			return VisualWitnessReceipt{}, statErr
		}
		screenshotBytes = info.Size()
		if screenshotBytes == 0 {
			return VisualWitnessReceipt{}, errors.New("qhtml visual witness rejected empty screenshot")
		}
	}

	viewport := strings.TrimSpace(req.Viewport)
	if viewport == "" {
		viewport = "unspecified"
	}
	stateRoot := visualWitnessRoot(projectRoot, req.StateRoot)
	visualDigest := sha256Hex([]byte(strings.Join([]string{
		"qhtml.visual_witness.v1",
		viewport,
		exportDigest,
		consoleDigest,
		screenshotDigest,
	}, "\n")))
	receiptPath := filepath.Join(stateRoot, visualDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_visual_witness.json")
	receipt := VisualWitnessReceipt{
		SchemaVersion:     VisualWitnessSchemaVersion,
		Status:            "visual_witness",
		ProductID:         "qhtml",
		ProjectRoot:       slashClean(projectRoot),
		ExportPath:        slashClean(exportPath),
		ConsoleReportPath: slashClean(consolePath),
		ScreenshotPath:    slashClean(screenshotPath),
		StateRoot:         slashClean(stateRoot),
		Viewport:          viewport,
		ExportDigest:      exportDigest,
		ConsoleDigest:     consoleDigest,
		ScreenshotDigest:  screenshotDigest,
		VisualDigest:      visualDigest,
		NonBlank:          nonBlank,
		ConsoleErrors:     consoleErrors,
		ScreenshotBytes:   screenshotBytes,
		Checks: []string{
			"export_file_digest",
			"nonblank_visible_text",
			"console_error_count_zero_when_report_present",
			"screenshot_nonzero_when_present",
		},
		NegativeCases: []string{
			"blank_export_rejected",
			"console_error_rejected",
			"empty_screenshot_rejected",
		},
		Policy:     "browser_artifacts_are_external; qhtml_seals_visual_evidence_without_neuronfs_runtime",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"witness_role": "binds rendered export to browser-side visual/console evidence",
		},
	}
	if req.WriteEvidence {
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func visualWitnessRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "visual_witnesses")
}

func htmlHasVisibleText(value string) bool {
	noScript := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`).ReplaceAllString(value, " ")
	noTags := regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(noScript, " ")
	cleaned := strings.TrimSpace(strings.Join(strings.Fields(noTags), " "))
	return cleaned != ""
}

func countConsoleErrors(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var values []map[string]any
	if err := json.Unmarshal(stripUTF8BOM(data), &values); err == nil {
		count := 0
		for _, item := range values {
			level := strings.ToLower(strings.TrimSpace(anyString(item["level"])))
			typ := strings.ToLower(strings.TrimSpace(anyString(item["type"])))
			if level == "error" || typ == "error" {
				count++
			}
		}
		return count, nil
	}
	var object map[string]any
	if err := json.Unmarshal(stripUTF8BOM(data), &object); err == nil {
		if value, ok := numberAsInt(object["errors"]); ok {
			return value, nil
		}
		if entries, ok := object["entries"].([]any); ok {
			count := 0
			for _, entry := range entries {
				if item, ok := entry.(map[string]any); ok {
					level := strings.ToLower(strings.TrimSpace(anyString(item["level"])))
					typ := strings.ToLower(strings.TrimSpace(anyString(item["type"])))
					if level == "error" || typ == "error" {
						count++
					}
				}
			}
			return count, nil
		}
		return 0, nil
	}
	text := strings.ToLower(string(data))
	return strings.Count(text, "error"), nil
}

func anyString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func numberAsInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

func stripUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}
