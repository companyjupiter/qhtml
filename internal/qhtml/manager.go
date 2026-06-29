package qhtml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ManageSchemaVersion = "qhtml.manage.v1"

type ManageRequest struct {
	ProjectRoot   string
	LaneRoot      string
	SourcePath    string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type ManageSnapshot struct {
	SchemaVersion        string            `json:"schema_version"`
	Status               string            `json:"status"`
	ProductID            string            `json:"product_id"`
	LaneRoot             string            `json:"lane_root"`
	SourcePath           string            `json:"source_path,omitempty"`
	StatePath            string            `json:"state_path"`
	ReceiptPath          string            `json:"receipt_path,omitempty"`
	LaneDigest           string            `json:"lane_digest"`
	PreviousLaneDigest   string            `json:"previous_lane_digest,omitempty"`
	SourceDigest         string            `json:"source_digest,omitempty"`
	PreviousSourceDigest string            `json:"previous_source_digest,omitempty"`
	LaneChanged          bool              `json:"lane_changed"`
	SourceChanged        bool              `json:"source_changed"`
	NeedsRenderRefresh   bool              `json:"needs_render_refresh"`
	FileCount            int               `json:"file_count"`
	DirCount             int               `json:"dir_count"`
	ChangedReasons       []string          `json:"changed_reasons"`
	ManagedBy            string            `json:"managed_by"`
	Policy               string            `json:"policy"`
	ObservedAt           string            `json:"observed_at"`
	Details              map[string]string `json:"details,omitempty"`
}

type managedState struct {
	SchemaVersion string `json:"schema_version"`
	LaneRoot      string `json:"lane_root"`
	SourcePath    string `json:"source_path,omitempty"`
	LaneDigest    string `json:"lane_digest"`
	SourceDigest  string `json:"source_digest,omitempty"`
	FileCount     int    `json:"file_count"`
	DirCount      int    `json:"dir_count"`
	UpdatedAt     string `json:"updated_at"`
}

func Manage(req ManageRequest) (ManageSnapshot, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return ManageSnapshot{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return ManageSnapshot{}, errors.New("qhtml lane root required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	laneDigest, fileCount, dirCount, err := digestTree(laneRoot)
	if err != nil {
		return ManageSnapshot{}, err
	}
	sourcePath := ""
	sourceDigest := ""
	if strings.TrimSpace(req.SourcePath) != "" {
		sourcePath = absPath(projectRoot, req.SourcePath)
		sourceDigest, err = digestFile(sourcePath)
		if err != nil {
			return ManageSnapshot{}, err
		}
	}
	statePath := managedStatePath(projectRoot, req.StateRoot, laneRoot, sourcePath)
	var previous managedState
	_ = readManagedState(statePath, &previous)

	laneChanged := previous.LaneDigest == "" || previous.LaneDigest != laneDigest
	sourceChanged := sourcePath != "" && (previous.SourceDigest == "" || previous.SourceDigest != sourceDigest)
	reasons := changeReasons(previous, laneDigest, sourcePath, sourceDigest)
	status := "current"
	if previous.LaneDigest == "" {
		status = "initialized"
	} else if laneChanged || sourceChanged {
		status = "changed"
	}
	snapshot := ManageSnapshot{
		SchemaVersion:        ManageSchemaVersion,
		Status:               status,
		ProductID:            "qhtml",
		LaneRoot:             slashClean(laneRoot),
		SourcePath:           slashClean(sourcePath),
		StatePath:            slashClean(statePath),
		LaneDigest:           laneDigest,
		PreviousLaneDigest:   previous.LaneDigest,
		SourceDigest:         sourceDigest,
		PreviousSourceDigest: previous.SourceDigest,
		LaneChanged:          laneChanged,
		SourceChanged:        sourceChanged,
		NeedsRenderRefresh:   laneChanged || sourceChanged,
		FileCount:            fileCount,
		DirCount:             dirCount,
		ChangedReasons:       reasons,
		ManagedBy:            "go_digest_refresh",
		Policy:               "folder_lane_and_source_digest_are_ssot; watcher_is_optimization_not_correctness",
		ObservedAt:           req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"state_model":  ".qhtml/managed deterministic state and receipts",
		},
	}
	if req.WriteEvidence {
		state := managedState{
			SchemaVersion: ManageSchemaVersion,
			LaneRoot:      snapshot.LaneRoot,
			SourcePath:    snapshot.SourcePath,
			LaneDigest:    laneDigest,
			SourceDigest:  sourceDigest,
			FileCount:     fileCount,
			DirCount:      dirCount,
			UpdatedAt:     snapshot.ObservedAt,
		}
		if err := writeJSON(statePath, state); err != nil {
			return snapshot, err
		}
		receiptPath := filepath.Join(filepath.Dir(statePath), "receipts", req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_refresh.json")
		if err := writeJSON(receiptPath, snapshot); err != nil {
			return snapshot, err
		}
		snapshot.ReceiptPath = slashClean(receiptPath)
	}
	return snapshot, nil
}

func digestTree(root string) (string, int, int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", 0, 0, err
	}
	if !info.IsDir() {
		return "", 0, 0, errors.New("qhtml lane root is not a directory")
	}
	var entries []string
	fileCount := 0
	dirCount := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			dirCount++
			entries = append(entries, "dir\t"+rel)
			return nil
		}
		digest, digestErr := digestFile(path)
		if digestErr != nil {
			return digestErr
		}
		fileCount++
		entries = append(entries, "file\t"+rel+"\t"+digest)
		return nil
	})
	if err != nil {
		return "", 0, 0, err
	}
	sort.Strings(entries)
	return sha256Hex([]byte(strings.Join(entries, "\n"))), fileCount, dirCount, nil
}

func digestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func managedStatePath(projectRoot, stateRoot, laneRoot, sourcePath string) string {
	root := stateRoot
	if strings.TrimSpace(root) == "" {
		root = filepath.Join(projectRoot, ".qhtml", "managed")
	} else {
		root = absPath(projectRoot, root)
	}
	key := laneRoot
	if sourcePath != "" {
		key += "|" + sourcePath
	}
	digest := sha256Hex([]byte(filepath.ToSlash(filepath.Clean(key))))[:16]
	return filepath.Join(root, digest, "state.json")
}

func readManagedState(path string, out *managedState) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func changeReasons(previous managedState, laneDigest, sourcePath, sourceDigest string) []string {
	reasons := []string{}
	if previous.LaneDigest == "" {
		reasons = append(reasons, "no_previous_state")
	} else if previous.LaneDigest != laneDigest {
		reasons = append(reasons, "lane_digest_changed")
	}
	if sourcePath != "" {
		if previous.SourceDigest == "" {
			reasons = append(reasons, "no_previous_source_state")
		} else if previous.SourceDigest != sourceDigest {
			reasons = append(reasons, "source_digest_changed")
		}
	}
	return reasons
}

func projectRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return os.Getwd()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func absPath(projectRoot, path string) string {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		return clean
	}
	return filepath.Join(projectRoot, clean)
}

func slashClean(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}
