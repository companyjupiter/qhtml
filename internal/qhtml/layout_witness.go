package qhtml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const LayoutWitnessSchemaVersion = "qhtml.layout_witness.v1"

type LayoutWitnessRequest struct {
	ProjectRoot   string
	ExportPath    string
	ReportPath    string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type LayoutReport struct {
	Viewports []LayoutViewport `json:"viewports"`
}

type LayoutViewport struct {
	Name          string   `json:"name"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	NonBlank      bool     `json:"nonblank"`
	ConsoleErrors int      `json:"console_errors"`
	OverflowX     int      `json:"overflow_x"`
	OverflowY     int      `json:"overflow_y"`
	Screenshot    string   `json:"screenshot,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

type LayoutWitnessReceipt struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	ProjectRoot   string            `json:"project_root"`
	ExportPath    string            `json:"export_path"`
	ReportPath    string            `json:"report_path"`
	StateRoot     string            `json:"state_root"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	ExportDigest  string            `json:"export_digest"`
	ReportDigest  string            `json:"report_digest"`
	LayoutDigest  string            `json:"layout_digest"`
	ViewportCount int               `json:"viewport_count"`
	Viewports     []LayoutViewport  `json:"viewports"`
	Checks        []string          `json:"checks"`
	NegativeCases []string          `json:"negative_cases"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func LayoutWitness(req LayoutWitnessRequest) (LayoutWitnessReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return LayoutWitnessReceipt{}, err
	}
	if strings.TrimSpace(req.ExportPath) == "" {
		return LayoutWitnessReceipt{}, errors.New("qhtml export path required for layout witness")
	}
	if strings.TrimSpace(req.ReportPath) == "" {
		return LayoutWitnessReceipt{}, errors.New("qhtml layout report path required")
	}
	exportPath := absPath(projectRoot, req.ExportPath)
	reportPath := absPath(projectRoot, req.ReportPath)
	exportDigest, err := digestFile(exportPath)
	if err != nil {
		return LayoutWitnessReceipt{}, err
	}
	reportDigest, err := digestFile(reportPath)
	if err != nil {
		return LayoutWitnessReceipt{}, err
	}
	report, err := readLayoutReport(reportPath)
	if err != nil {
		return LayoutWitnessReceipt{}, err
	}
	if err := validateLayoutReport(report); err != nil {
		return LayoutWitnessReceipt{}, err
	}
	stateRoot := layoutWitnessRoot(projectRoot, req.StateRoot)
	layoutDigest := sha256Hex([]byte(strings.Join([]string{
		"qhtml.layout_witness.v1",
		exportDigest,
		reportDigest,
	}, "\n")))
	receiptPath := filepath.Join(stateRoot, layoutDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_layout_witness.json")
	receipt := LayoutWitnessReceipt{
		SchemaVersion: LayoutWitnessSchemaVersion,
		Status:        "layout_witness",
		ProductID:     "qhtml",
		ProjectRoot:   slashClean(projectRoot),
		ExportPath:    slashClean(exportPath),
		ReportPath:    slashClean(reportPath),
		StateRoot:     slashClean(stateRoot),
		ExportDigest:  exportDigest,
		ReportDigest:  reportDigest,
		LayoutDigest:  layoutDigest,
		ViewportCount: len(report.Viewports),
		Viewports:     report.Viewports,
		Checks: []string{
			"layout_report_has_viewports",
			"each_viewport_nonblank",
			"each_viewport_console_errors_zero",
			"each_viewport_overflow_zero",
			"each_viewport_has_positive_dimensions",
		},
		NegativeCases: []string{
			"missing_viewport_rejected",
			"blank_viewport_rejected",
			"console_error_rejected",
			"overflow_rejected",
			"invalid_dimensions_rejected",
		},
		Policy:     "browser_layout_runner_is_external; qhtml_seals_responsive_overflow_evidence_without_neuronfs_runtime",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"witness_role": "binds rendered export to browser layout report across viewports",
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

func readLayoutReport(path string) (LayoutReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LayoutReport{}, err
	}
	var report LayoutReport
	if err := json.Unmarshal(stripUTF8BOM(data), &report); err == nil && len(report.Viewports) > 0 {
		return report, nil
	}
	var viewports []LayoutViewport
	if err := json.Unmarshal(stripUTF8BOM(data), &viewports); err != nil {
		return LayoutReport{}, err
	}
	return LayoutReport{Viewports: viewports}, nil
}

func validateLayoutReport(report LayoutReport) error {
	if len(report.Viewports) == 0 {
		return errors.New("qhtml layout witness rejected missing viewports")
	}
	for _, viewport := range report.Viewports {
		name := strings.TrimSpace(viewport.Name)
		if name == "" {
			name = "unnamed"
		}
		if viewport.Width <= 0 || viewport.Height <= 0 {
			return errors.New("qhtml layout witness rejected invalid viewport dimensions: " + name)
		}
		if !viewport.NonBlank {
			return errors.New("qhtml layout witness rejected blank viewport: " + name)
		}
		if viewport.ConsoleErrors > 0 {
			return errors.New("qhtml layout witness rejected console errors: " + name)
		}
		if viewport.OverflowX > 0 || viewport.OverflowY > 0 {
			return errors.New("qhtml layout witness rejected overflow: " + name)
		}
	}
	return nil
}

func layoutWitnessRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "layout_witnesses")
}
