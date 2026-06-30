package qhtml

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
)

const WitnessSchemaVersion = "qhtml.witness.v1"

type WitnessRequest struct {
	ProjectRoot   string
	LaneRoot      string
	SourcePath    string
	ExportPath    string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type WitnessReceipt struct {
	SchemaVersion     string            `json:"schema_version"`
	Status            string            `json:"status"`
	ProductID         string            `json:"product_id"`
	LaneRoot          string            `json:"lane_root"`
	SourcePath        string            `json:"source_path,omitempty"`
	ExportPath        string            `json:"export_path"`
	StateRoot         string            `json:"state_root"`
	WitnessPath       string            `json:"witness_path,omitempty"`
	LaneDigest        string            `json:"lane_digest"`
	SourceDigest      string            `json:"source_digest,omitempty"`
	ExportDigest      string            `json:"export_digest"`
	RenderInputDigest string            `json:"render_input_digest"`
	FileCount         int               `json:"file_count"`
	DirCount          int               `json:"dir_count"`
	SymlinkCount      int               `json:"symlink_count"`
	NegativeCases     []string          `json:"negative_cases"`
	Policy            string            `json:"policy"`
	ObservedAt        string            `json:"observed_at"`
	Details           map[string]string `json:"details,omitempty"`
}

func Witness(req WitnessRequest) (WitnessReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return WitnessReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return WitnessReceipt{}, errors.New("qhtml lane root required")
	}
	if strings.TrimSpace(req.ExportPath) == "" {
		return WitnessReceipt{}, errors.New("qhtml export path required for render witness")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	sourcePath := ""
	sourceDigest := ""
	if strings.TrimSpace(req.SourcePath) != "" {
		sourcePath = absPath(projectRoot, req.SourcePath)
		sourceDigest, err = digestFile(sourcePath)
		if err != nil {
			return WitnessReceipt{}, err
		}
	}
	exportPath := absPath(projectRoot, req.ExportPath)
	exportDigest, err := digestFile(exportPath)
	if err != nil {
		return WitnessReceipt{}, err
	}
	stateRoot := witnessRoot(projectRoot, req.StateRoot)
	laneDigest, fileCount, dirCount, symlinkCount, ignored, err := digestTree(laneRoot, []string{stateRoot})
	if err != nil {
		return WitnessReceipt{}, err
	}
	renderInputDigest := sha256Hex([]byte(strings.Join([]string{
		"qhtml.render_input.v1",
		laneDigest,
		sourceDigest,
		exportDigest,
	}, "\n")))
	witnessPath := filepath.Join(stateRoot, renderInputDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_witness.json")
	receipt := WitnessReceipt{
		SchemaVersion:     WitnessSchemaVersion,
		Status:            "render_witness",
		ProductID:         "qhtml",
		LaneRoot:          slashClean(laneRoot),
		SourcePath:        slashClean(sourcePath),
		ExportPath:        slashClean(exportPath),
		StateRoot:         slashClean(stateRoot),
		LaneDigest:        laneDigest,
		SourceDigest:      sourceDigest,
		ExportDigest:      exportDigest,
		RenderInputDigest: renderInputDigest,
		FileCount:         fileCount,
		DirCount:          dirCount,
		SymlinkCount:      symlinkCount,
		NegativeCases: []string{
			"missing_export_rejected",
			"missing_lane_rejected",
			"export_digest_changes_when_render_changes",
		},
		Policy:     "folder_lane_source_and_export_digest_are_render_witness_ssot; standalone_qhtml_no_neuronfs_runtime_required",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"ignored":      strings.Join(ignored, ","),
			"witness_role": "binds QHTML lane/source digest to rendered HTML export digest",
		},
	}
	if req.WriteEvidence {
		if err := writeJSON(witnessPath, receipt); err != nil {
			return receipt, err
		}
		receipt.WitnessPath = slashClean(witnessPath)
	}
	return receipt, nil
}

func witnessRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "witnesses")
}
