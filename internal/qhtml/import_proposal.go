package qhtml

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ImportProposalSchemaVersion = "qhtml.import_proposal.v1"

type ImportProposalRequest struct {
	ProjectRoot   string
	LaneRoot      string
	ExportPath    string
	TargetPath    string
	Kind          string
	SourceReceipt string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type ImportProposalReceipt struct {
	SchemaVersion  string            `json:"schema_version"`
	Status         string            `json:"status"`
	ProductID      string            `json:"product_id"`
	LaneRoot       string            `json:"lane_root"`
	ExportPath     string            `json:"export_path"`
	TargetPath     string            `json:"target_path,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	LaneDigest     string            `json:"lane_digest"`
	TargetDigest   string            `json:"target_digest,omitempty"`
	ExportDigest   string            `json:"export_digest"`
	ProposalDigest string            `json:"proposal_digest"`
	SourceReceipt  string            `json:"source_receipt,omitempty"`
	ReceiptPath    string            `json:"receipt_path,omitempty"`
	PatchAction    string            `json:"patch_action"`
	ManagedBy      string            `json:"managed_by"`
	Policy         string            `json:"policy"`
	ObservedAt     string            `json:"observed_at"`
	Details        map[string]string `json:"details,omitempty"`
}

func ImportProposal(req ImportProposalRequest) (ImportProposalReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return ImportProposalReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return ImportProposalReceipt{}, errors.New("qhtml lane root required")
	}
	if strings.TrimSpace(req.ExportPath) == "" {
		return ImportProposalReceipt{}, errors.New("qhtml import-proposal --export required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	exportPath := absPath(projectRoot, req.ExportPath)
	exportDigest, err := digestFile(exportPath)
	if err != nil {
		return ImportProposalReceipt{}, err
	}
	exportData, err := os.ReadFile(exportPath)
	if err != nil {
		return ImportProposalReceipt{}, err
	}
	if !htmlHasVisibleText(string(exportData)) {
		return ImportProposalReceipt{}, errors.New("qhtml import-proposal rejected blank export")
	}
	laneDigest, fileCount, dirCount, symlinkCount, _, err := digestTree(laneRoot, nil)
	if err != nil {
		return ImportProposalReceipt{}, err
	}
	targetPath := ""
	targetDigest := ""
	kind := ""
	if strings.TrimSpace(req.TargetPath) != "" {
		target, targetErr := Target(TargetRequest{
			ProjectRoot: req.ProjectRoot,
			LaneRoot:    req.LaneRoot,
			TargetPath:  req.TargetPath,
			Kind:        req.Kind,
			ObservedAt:  req.ObservedAt,
		})
		if targetErr != nil {
			return ImportProposalReceipt{}, targetErr
		}
		targetPath = target.TargetPath
		targetDigest = target.Digest
		kind = target.Kind
	}
	proposalInput := strings.Join([]string{
		slashClean(laneRoot),
		slashClean(exportPath),
		targetPath,
		kind,
		laneDigest,
		targetDigest,
		exportDigest,
		slashClean(req.SourceReceipt),
	}, "\n")
	proposalDigest := sha256Hex([]byte(proposalInput))
	receipt := ImportProposalReceipt{
		SchemaVersion:  ImportProposalSchemaVersion,
		Status:         "import_proposal",
		ProductID:      "qhtml",
		LaneRoot:       slashClean(laneRoot),
		ExportPath:     slashClean(exportPath),
		TargetPath:     targetPath,
		Kind:           kind,
		LaneDigest:     laneDigest,
		TargetDigest:   targetDigest,
		ExportDigest:   exportDigest,
		ProposalDigest: proposalDigest,
		SourceReceipt:  slashClean(req.SourceReceipt),
		PatchAction:    "proposal_only",
		ManagedBy:      "go_import_proposal",
		Policy:         "export_changes_become_lane_patch_proposals; no_direct_lane_overwrite",
		ObservedAt:     req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root":  projectRoot,
			"state_model":   ".qhtml/import_proposals deterministic receipts",
			"file_count":    intString(fileCount),
			"dir_count":     intString(dirCount),
			"symlink_count": intString(symlinkCount),
		},
	}
	if req.WriteEvidence {
		receiptPath := importProposalReceiptPath(projectRoot, req.StateRoot, laneRoot, exportPath, targetPath, req.ObservedAt)
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func importProposalReceiptPath(projectRootInput, stateRoot, laneRoot, exportPath, targetPath string, observedAt time.Time) string {
	root := strings.TrimSpace(stateRoot)
	if root == "" {
		root = filepath.Join(projectRootInput, ".qhtml", "import_proposals")
	} else {
		root = absPath(projectRootInput, root)
	}
	key := sha256Hex([]byte(filepath.ToSlash(filepath.Clean(laneRoot + "|" + exportPath + "|" + targetPath))))[:16]
	return filepath.Join(root, key, observedAt.UTC().Format("20060102T150405Z")+".qhtml_import_proposal.json")
}

func intString(value int) string {
	return strconv.Itoa(value)
}
