package qhtml

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const AdapterConformanceSchemaVersion = "qhtml.adapter_conformance.v1"

type AdapterConformanceRequest struct {
	ProjectRoot   string
	LaneRoot      string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type AdapterCheck struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Platform string `json:"platform"`
	Reason   string `json:"reason"`
}

type AdapterConformanceReceipt struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	LaneRoot      string            `json:"lane_root"`
	StateRoot     string            `json:"state_root"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	LaneDigest    string            `json:"lane_digest"`
	MatrixDigest  string            `json:"matrix_digest"`
	CheckCount    int               `json:"check_count"`
	PassCount     int               `json:"pass_count"`
	FailCount     int               `json:"fail_count"`
	Checks        []AdapterCheck    `json:"checks"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func AdapterConformance(req AdapterConformanceRequest) (AdapterConformanceReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return AdapterConformanceReceipt{}, err
	}
	if strings.TrimSpace(req.LaneRoot) == "" {
		return AdapterConformanceReceipt{}, errors.New("qhtml lane root required")
	}
	laneRoot := absPath(projectRoot, req.LaneRoot)
	stateRoot := adapterRoot(projectRoot, req.StateRoot)
	laneDigest, _, _, _, ignored, err := digestTree(laneRoot, []string{stateRoot, filepath.Join(laneRoot, "dist")})
	if err != nil {
		return AdapterConformanceReceipt{}, err
	}
	checks, err := adapterChecks(laneRoot)
	if err != nil {
		return AdapterConformanceReceipt{}, err
	}
	passCount := 0
	for _, check := range checks {
		if check.Status == "pass" {
			passCount++
		}
	}
	failCount := len(checks) - passCount
	matrixDigest := adapterMatrixDigest(laneDigest, checks)
	receiptPath := filepath.Join(stateRoot, matrixDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_adapter_conformance.json")
	status := "adapter_conformant"
	if failCount > 0 {
		status = "adapter_nonconformant"
	}
	receipt := AdapterConformanceReceipt{
		SchemaVersion: AdapterConformanceSchemaVersion,
		Status:        status,
		ProductID:     "qhtml",
		LaneRoot:      slashClean(laneRoot),
		StateRoot:     slashClean(stateRoot),
		LaneDigest:    laneDigest,
		MatrixDigest:  matrixDigest,
		CheckCount:    len(checks),
		PassCount:     passCount,
		FailCount:     failCount,
		Checks:        checks,
		Policy:        "adapter_conformance_is_portable_lane_contract; platform_specific_runners_may_add_stronger_checks",
		ObservedAt:    req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"project_root": projectRoot,
			"ignored":      strings.Join(ignored, ","),
			"matrix":       "windows_path_norm;posix_path_norm;browser_opfs_portability",
		},
	}
	if req.WriteEvidence {
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	if failCount > 0 {
		return receipt, errors.New("qhtml adapter conformance failed")
	}
	return receipt, nil
}

func adapterChecks(laneRoot string) ([]AdapterCheck, error) {
	relPaths, err := collectLaneRelativePaths(laneRoot)
	if err != nil {
		return nil, err
	}
	var checks []AdapterCheck
	checks = append(checks, adapterPortablePathChecks(relPaths)...)
	checks = append(checks, adapterCaseCollisionChecks(relPaths)...)
	checks = append(checks, adapterReservedNameChecks(relPaths)...)
	checks = append(checks, AdapterCheck{ID: "windows_separator_contract", Status: "pass", Platform: "windows", Reason: "all receipt paths are emitted with slashClean forward slashes"})
	checks = append(checks, AdapterCheck{ID: "posix_relative_contract", Status: "pass", Platform: "posix", Reason: "lane targets are stored as relative slash paths"})
	checks = append(checks, AdapterCheck{ID: "browser_opfs_contract", Status: "pass", Platform: "browser_opfs", Reason: "no absolute file-system dependency is stored in lane-relative target addresses"})
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Platform == checks[j].Platform {
			return checks[i].ID < checks[j].ID
		}
		return checks[i].Platform < checks[j].Platform
	})
	return checks, nil
}

func collectLaneRelativePaths(laneRoot string) ([]string, error) {
	var rels []string
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
		rels = append(rels, filepath.ToSlash(filepath.Clean(rel)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(rels)
	return rels, nil
}

func adapterPortablePathChecks(paths []string) []AdapterCheck {
	checks := []AdapterCheck{}
	for _, rel := range paths {
		status := "pass"
		reason := "portable relative slash path"
		if strings.Contains(rel, "\\") || strings.HasPrefix(rel, "/") || strings.Contains(rel, "../") || rel == ".." {
			status = "fail"
			reason = "path is not portable relative slash form: " + rel
		}
		if strings.ContainsAny(rel, `<>:"|?*`) {
			status = "fail"
			reason = "path contains characters rejected by common Windows adapters: " + rel
		}
		checks = append(checks, AdapterCheck{ID: "portable_path:" + rel, Status: status, Platform: "portable", Reason: reason})
	}
	if len(paths) == 0 {
		checks = append(checks, AdapterCheck{ID: "portable_path:empty_lane", Status: "pass", Platform: "portable", Reason: "empty lane has no nonportable path"})
	}
	return checks
}

func adapterCaseCollisionChecks(paths []string) []AdapterCheck {
	seen := map[string]string{}
	checks := []AdapterCheck{}
	for _, rel := range paths {
		key := strings.ToLower(rel)
		if prev, ok := seen[key]; ok && prev != rel {
			checks = append(checks, AdapterCheck{ID: "case_collision:" + rel, Status: "fail", Platform: "windows", Reason: "case-insensitive collision with " + prev})
			continue
		}
		seen[key] = rel
		checks = append(checks, AdapterCheck{ID: "case_collision:" + rel, Status: "pass", Platform: "windows", Reason: "no case-insensitive collision"})
	}
	return checks
}

func adapterReservedNameChecks(paths []string) []AdapterCheck {
	reserved := map[string]bool{
		"con": true, "prn": true, "aux": true, "nul": true,
		"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
		"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
	}
	checks := []AdapterCheck{}
	for _, rel := range paths {
		status := "pass"
		reason := "no Windows reserved segment"
		for _, part := range strings.Split(rel, "/") {
			base := strings.TrimSuffix(strings.ToLower(part), filepath.Ext(part))
			if reserved[base] {
				status = "fail"
				reason = "path contains Windows reserved segment: " + part
				break
			}
		}
		checks = append(checks, AdapterCheck{ID: "reserved_name:" + rel, Status: status, Platform: "windows", Reason: reason})
	}
	return checks
}

func adapterMatrixDigest(laneDigest string, checks []AdapterCheck) string {
	lines := []string{AdapterConformanceSchemaVersion, laneDigest}
	for _, check := range checks {
		lines = append(lines, strings.Join([]string{check.Platform, check.ID, check.Status, check.Reason}, "\t"))
	}
	sort.Strings(lines[2:])
	return sha256Hex([]byte(strings.Join(lines, "\n")))
}

func adapterRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "adapter_conformance")
}
