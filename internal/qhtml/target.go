package qhtml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const TargetSchemaVersion = "qhtml.target.v1"

type TargetRequest struct {
	ProjectRoot   string
	LaneRoot      string
	TargetPath    string
	Kind          string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type TargetSnapshot struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	TargetPath    string            `json:"target_path"`
	Kind          string            `json:"kind"`
	Exists        bool              `json:"exists"`
	IsDir         bool              `json:"is_dir"`
	Digest        string            `json:"digest"`
	FileCount     int               `json:"file_count,omitempty"`
	DirCount      int               `json:"dir_count,omitempty"`
	SymlinkCount  int               `json:"symlink_count,omitempty"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	ManagedBy     string            `json:"managed_by"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

type TombstoneRequest struct {
	ProjectRoot   string
	LaneRoot      string
	TargetPath    string
	Kind          string
	Reason        string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type TombstoneSnapshot struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	TargetPath    string            `json:"target_path"`
	Kind          string            `json:"kind"`
	Reason        string            `json:"reason"`
	TargetDigest  string            `json:"target_digest"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	ManagedBy     string            `json:"managed_by"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

type RollbackRequest struct {
	ProjectRoot   string
	LaneRoot      string
	TargetPath    string
	Kind          string
	ToDigest      string
	SourceReceipt string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type RollbackSnapshot struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	TargetPath    string            `json:"target_path"`
	Kind          string            `json:"kind"`
	CurrentDigest string            `json:"current_digest"`
	ToDigest      string            `json:"to_digest"`
	SourceReceipt string            `json:"source_receipt,omitempty"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	ManagedBy     string            `json:"managed_by"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func Target(req TargetRequest) (TargetSnapshot, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, laneRoot, rel, targetAbs, err := resolveTarget(req.ProjectRoot, req.LaneRoot, req.TargetPath)
	if err != nil {
		return TargetSnapshot{}, err
	}
	info, err := os.Lstat(targetAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TargetSnapshot{}, errors.New("qhtml target path does not exist")
		}
		return TargetSnapshot{}, err
	}
	digest, files, dirs, symlinks, err := digestTarget(targetAbs, info)
	if err != nil {
		return TargetSnapshot{}, err
	}
	snapshot := TargetSnapshot{
		SchemaVersion: TargetSchemaVersion,
		Status:        "target",
		ProductID:     "qhtml",
		LaneRoot:      slashClean(laneRoot),
		TargetPath:    rel,
		Kind:          targetKind(req.Kind),
		Exists:        true,
		IsDir:         info.IsDir(),
		Digest:        digest,
		FileCount:     files,
		DirCount:      dirs,
		SymlinkCount:  symlinks,
		ManagedBy:     "go_target_resolver",
		Policy:        "lane_relative_target_only; generated_html_is_not_source_truth",
		ObservedAt:    req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"target_abs":   slashClean(targetAbs),
			"state_model":  ".qhtml/targets deterministic receipts",
		},
	}
	if req.WriteEvidence {
		receiptPath := targetReceiptPath(projectRoot, req.StateRoot, laneRoot, rel, req.ObservedAt, "target")
		if err := writeJSON(receiptPath, snapshot); err != nil {
			return snapshot, err
		}
		snapshot.ReceiptPath = slashClean(receiptPath)
	}
	return snapshot, nil
}

func Tombstone(req TombstoneRequest) (TombstoneSnapshot, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	target, err := Target(TargetRequest{
		ProjectRoot: req.ProjectRoot,
		LaneRoot:    req.LaneRoot,
		TargetPath:  req.TargetPath,
		Kind:        req.Kind,
		ObservedAt:  req.ObservedAt,
	})
	if err != nil {
		return TombstoneSnapshot{}, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "unspecified"
	}
	projectRoot, _ := projectRoot(req.ProjectRoot)
	snapshot := TombstoneSnapshot{
		SchemaVersion: TargetSchemaVersion,
		Status:        "tombstone_proposal",
		ProductID:     "qhtml",
		LaneRoot:      target.LaneRoot,
		TargetPath:    target.TargetPath,
		Kind:          target.Kind,
		Reason:        reason,
		TargetDigest:  target.Digest,
		ManagedBy:     "go_target_resolver",
		Policy:        "tombstone_is_receipt_first; no_lane_delete_without_external_promotion_gate",
		ObservedAt:    req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"state_model":  ".qhtml/targets deterministic receipts",
		},
	}
	if req.WriteEvidence {
		receiptPath := targetReceiptPath(projectRoot, req.StateRoot, target.LaneRoot, target.TargetPath, req.ObservedAt, "tombstone")
		if err := writeJSON(receiptPath, snapshot); err != nil {
			return snapshot, err
		}
		snapshot.ReceiptPath = slashClean(receiptPath)
	}
	return snapshot, nil
}

func Rollback(req RollbackRequest) (RollbackSnapshot, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	if strings.TrimSpace(req.ToDigest) == "" {
		return RollbackSnapshot{}, errors.New("qhtml rollback --to-digest required")
	}
	target, err := Target(TargetRequest{
		ProjectRoot: req.ProjectRoot,
		LaneRoot:    req.LaneRoot,
		TargetPath:  req.TargetPath,
		Kind:        req.Kind,
		ObservedAt:  req.ObservedAt,
	})
	if err != nil {
		return RollbackSnapshot{}, err
	}
	projectRoot, _ := projectRoot(req.ProjectRoot)
	snapshot := RollbackSnapshot{
		SchemaVersion: TargetSchemaVersion,
		Status:        "rollback_proposal",
		ProductID:     "qhtml",
		LaneRoot:      target.LaneRoot,
		TargetPath:    target.TargetPath,
		Kind:          target.Kind,
		CurrentDigest: target.Digest,
		ToDigest:      strings.TrimSpace(req.ToDigest),
		SourceReceipt: slashClean(req.SourceReceipt),
		ManagedBy:     "go_target_resolver",
		Policy:        "rollback_is_proposal_first; lane_mutation_requires_external_promotion_gate",
		ObservedAt:    req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"state_model":  ".qhtml/targets deterministic receipts",
		},
	}
	if req.WriteEvidence {
		receiptPath := targetReceiptPath(projectRoot, req.StateRoot, target.LaneRoot, target.TargetPath, req.ObservedAt, "rollback")
		if err := writeJSON(receiptPath, snapshot); err != nil {
			return snapshot, err
		}
		snapshot.ReceiptPath = slashClean(receiptPath)
	}
	return snapshot, nil
}

func resolveTarget(projectRootInput, laneRootInput, targetPathInput string) (string, string, string, string, error) {
	projectRoot, err := projectRoot(projectRootInput)
	if err != nil {
		return "", "", "", "", err
	}
	if strings.TrimSpace(laneRootInput) == "" {
		return "", "", "", "", errors.New("qhtml lane root required")
	}
	if strings.TrimSpace(targetPathInput) == "" {
		return "", "", "", "", errors.New("qhtml target path required")
	}
	laneRoot := absPath(projectRoot, laneRootInput)
	targetAbs := absPath(laneRoot, targetPathInput)
	rel, err := filepath.Rel(laneRoot, targetAbs)
	if err != nil {
		return "", "", "", "", err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", "", "", errors.New("qhtml target must be inside lane root")
	}
	return projectRoot, laneRoot, rel, targetAbs, nil
}

func digestTarget(path string, info os.FileInfo) (string, int, int, int, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", 0, 0, 0, err
		}
		return sha256Hex([]byte("symlink\t" + target)), 0, 0, 1, nil
	}
	if info.IsDir() {
		digest, files, dirs, symlinks, _, err := digestTree(path, nil)
		return digest, files, dirs, symlinks, err
	}
	digest, err := digestFile(path)
	if err != nil {
		return "", 0, 0, 0, err
	}
	return digest, 1, 0, 0, nil
}

func targetKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "cell"
	}
	return kind
}

func targetReceiptPath(projectRootInput, stateRoot, laneRoot, targetPath string, observedAt time.Time, action string) string {
	root := strings.TrimSpace(stateRoot)
	if root == "" {
		root = filepath.Join(projectRootInput, ".qhtml", "targets")
	} else {
		root = absPath(projectRootInput, root)
	}
	key := sha256Hex([]byte(filepath.ToSlash(filepath.Clean(laneRoot + "|" + targetPath))))[:16]
	return filepath.Join(root, key, action+"s", observedAt.UTC().Format("20060102T150405Z")+".qhtml_"+action+".json")
}
