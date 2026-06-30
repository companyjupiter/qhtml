package qhtml

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const OPFSProofSchemaVersion = "qhtml.opfs_proof.v1"

type OPFSProofRequest struct {
	ProjectRoot   string
	ReportPath    string
	RunnerID      string
	RunnerVersion string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type OPFSReport struct {
	OPFSAvailable   bool     `json:"opfs_available"`
	QuotaBytes      int64    `json:"quota_bytes"`
	FileHandle      bool     `json:"file_handle"`
	WriteReadDelete bool     `json:"write_read_delete"`
	PathRoundtrip   bool     `json:"path_roundtrip"`
	RelativePaths   bool     `json:"relative_paths"`
	ConsoleErrors   int      `json:"console_errors"`
	Browser         string   `json:"browser,omitempty"`
	Origin          string   `json:"origin,omitempty"`
	Notes           []string `json:"notes,omitempty"`
}

type OPFSProofReceipt struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProductID     string            `json:"product_id"`
	ProjectRoot   string            `json:"project_root"`
	ReportPath    string            `json:"report_path"`
	StateRoot     string            `json:"state_root"`
	ReceiptPath   string            `json:"receipt_path,omitempty"`
	RunnerID      string            `json:"runner_id"`
	RunnerVersion string            `json:"runner_version"`
	ReportDigest  string            `json:"report_digest"`
	ProofDigest   string            `json:"proof_digest"`
	Report        OPFSReport        `json:"report"`
	Checks        []string          `json:"checks"`
	NegativeCases []string          `json:"negative_cases"`
	Policy        string            `json:"policy"`
	ObservedAt    string            `json:"observed_at"`
	Details       map[string]string `json:"details,omitempty"`
}

func OPFSProof(req OPFSProofRequest) (OPFSProofReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return OPFSProofReceipt{}, err
	}
	if strings.TrimSpace(req.ReportPath) == "" {
		return OPFSProofReceipt{}, errors.New("qhtml opfs-proof --report required")
	}
	runnerID := strings.TrimSpace(req.RunnerID)
	if runnerID == "" {
		return OPFSProofReceipt{}, errors.New("qhtml opfs-proof --runner-id required")
	}
	runnerVersion := strings.TrimSpace(req.RunnerVersion)
	if runnerVersion == "" {
		return OPFSProofReceipt{}, errors.New("qhtml opfs-proof --runner-version required")
	}
	reportPath := absPath(projectRoot, req.ReportPath)
	reportDigest, err := digestFile(reportPath)
	if err != nil {
		return OPFSProofReceipt{}, err
	}
	report, err := readOPFSReport(reportPath)
	if err != nil {
		return OPFSProofReceipt{}, err
	}
	if err := validateOPFSReport(report); err != nil {
		return OPFSProofReceipt{}, err
	}
	proofDigest := sha256Hex([]byte(strings.Join([]string{
		OPFSProofSchemaVersion,
		runnerID,
		runnerVersion,
		reportDigest,
	}, "\n")))
	stateRoot := opfsProofRoot(projectRoot, req.StateRoot)
	receipt := OPFSProofReceipt{
		SchemaVersion: OPFSProofSchemaVersion,
		Status:        "opfs_proof",
		ProductID:     "qhtml",
		ProjectRoot:   slashClean(projectRoot),
		ReportPath:    slashClean(reportPath),
		StateRoot:     slashClean(stateRoot),
		RunnerID:      runnerID,
		RunnerVersion: runnerVersion,
		ReportDigest:  reportDigest,
		ProofDigest:   proofDigest,
		Report:        report,
		Checks: []string{
			"opfs_available",
			"quota_positive",
			"file_handle_available",
			"write_read_delete_roundtrip",
			"path_roundtrip",
			"relative_paths",
			"console_errors_zero",
		},
		NegativeCases: []string{
			"missing_report_rejected",
			"missing_runner_id_rejected",
			"opfs_unavailable_rejected",
			"zero_quota_rejected",
			"file_handle_missing_rejected",
			"roundtrip_failure_rejected",
			"console_errors_rejected",
		},
		Policy:     "browser_opfs_support_requires_external_runner_report; adapter_matrix_alone_is_not_browser_ready_claim",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"proof_role": "binds browser OPFS runner evidence before browser-ready handoff claims",
		},
	}
	if req.WriteEvidence {
		receiptPath := filepath.Join(stateRoot, proofDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_opfs_proof.json")
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func readOPFSReport(path string) (OPFSReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OPFSReport{}, err
	}
	var report OPFSReport
	if err := json.Unmarshal(stripUTF8BOM(data), &report); err != nil {
		return OPFSReport{}, err
	}
	return report, nil
}

func validateOPFSReport(report OPFSReport) error {
	if !report.OPFSAvailable {
		return errors.New("qhtml opfs-proof rejected unavailable OPFS")
	}
	if report.QuotaBytes <= 0 {
		return errors.New("qhtml opfs-proof rejected nonpositive quota")
	}
	if !report.FileHandle {
		return errors.New("qhtml opfs-proof rejected missing file handle support")
	}
	if !report.WriteReadDelete {
		return errors.New("qhtml opfs-proof rejected failed write/read/delete roundtrip")
	}
	if !report.PathRoundtrip {
		return errors.New("qhtml opfs-proof rejected failed path roundtrip")
	}
	if !report.RelativePaths {
		return errors.New("qhtml opfs-proof rejected non-relative OPFS paths")
	}
	if report.ConsoleErrors > 0 {
		return errors.New("qhtml opfs-proof rejected console errors")
	}
	return nil
}

func opfsProofRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "opfs_proofs")
}
