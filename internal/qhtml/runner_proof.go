package qhtml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RunnerProofSchemaVersion = "qhtml.runner_proof.v1"

type RunnerProofRequest struct {
	ProjectRoot   string
	ReportPath    string
	RunnerID      string
	RunnerVersion string
	Signature     string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type RunnerProofReceipt struct {
	SchemaVersion    string            `json:"schema_version"`
	Status           string            `json:"status"`
	ProductID        string            `json:"product_id"`
	ProjectRoot      string            `json:"project_root"`
	ReportPath       string            `json:"report_path"`
	StateRoot        string            `json:"state_root"`
	ReceiptPath      string            `json:"receipt_path,omitempty"`
	RunnerID         string            `json:"runner_id"`
	RunnerVersion    string            `json:"runner_version"`
	ReportDigest     string            `json:"report_digest"`
	SignaturePayload string            `json:"signature_payload"`
	Signature        string            `json:"signature"`
	ProofDigest      string            `json:"proof_digest"`
	Checks           []string          `json:"checks"`
	NegativeCases    []string          `json:"negative_cases"`
	ManagedBy        string            `json:"managed_by"`
	Policy           string            `json:"policy"`
	ObservedAt       string            `json:"observed_at"`
	Details          map[string]string `json:"details,omitempty"`
}

func RunnerProof(req RunnerProofRequest) (RunnerProofReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return RunnerProofReceipt{}, err
	}
	if strings.TrimSpace(req.ReportPath) == "" {
		return RunnerProofReceipt{}, errors.New("qhtml runner-proof --report required")
	}
	runnerID := strings.TrimSpace(req.RunnerID)
	if runnerID == "" {
		return RunnerProofReceipt{}, errors.New("qhtml runner-proof --runner-id required")
	}
	runnerVersion := strings.TrimSpace(req.RunnerVersion)
	if runnerVersion == "" {
		return RunnerProofReceipt{}, errors.New("qhtml runner-proof --runner-version required")
	}
	signature := strings.TrimSpace(req.Signature)
	if len(signature) < 16 {
		return RunnerProofReceipt{}, errors.New("qhtml runner-proof --signature must be at least 16 characters")
	}
	reportPath := absPath(projectRoot, req.ReportPath)
	info, err := os.Stat(reportPath)
	if err != nil {
		return RunnerProofReceipt{}, err
	}
	if info.IsDir() || info.Size() == 0 {
		return RunnerProofReceipt{}, errors.New("qhtml runner-proof rejected empty report")
	}
	reportDigest, err := digestFile(reportPath)
	if err != nil {
		return RunnerProofReceipt{}, err
	}
	signaturePayload := runnerProofSignaturePayload(runnerID, runnerVersion, reportDigest)
	proofDigest := sha256Hex([]byte(signaturePayload + "\n" + signature))
	stateRoot := runnerProofRoot(projectRoot, req.StateRoot)
	receipt := RunnerProofReceipt{
		SchemaVersion:    RunnerProofSchemaVersion,
		Status:           "runner_proof",
		ProductID:        "qhtml",
		ProjectRoot:      slashClean(projectRoot),
		ReportPath:       slashClean(reportPath),
		StateRoot:        slashClean(stateRoot),
		RunnerID:         runnerID,
		RunnerVersion:    runnerVersion,
		ReportDigest:     reportDigest,
		SignaturePayload: signaturePayload,
		Signature:        signature,
		ProofDigest:      proofDigest,
		Checks: []string{
			"runner_id_required",
			"runner_version_required",
			"report_digest_bound",
			"signature_required",
			"proof_digest_stable",
		},
		NegativeCases: []string{
			"missing_report_rejected",
			"empty_report_rejected",
			"missing_runner_id_rejected",
			"short_signature_rejected",
		},
		ManagedBy:  "go_runner_proof",
		Policy:     "browser_runner_report_must_carry_runner_identity_version_and_signature_claim",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"proof_role": "binds browser runner identity, report digest, and signature claim before promotion seal",
		},
	}
	if req.WriteEvidence {
		receiptPath := filepath.Join(stateRoot, proofDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_runner_proof.json")
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func runnerProofRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "runner_proofs")
}

func runnerProofSignaturePayload(runnerID, runnerVersion, reportDigest string) string {
	return strings.Join([]string{
		RunnerProofSchemaVersion,
		strings.TrimSpace(runnerID),
		strings.TrimSpace(runnerVersion),
		strings.TrimSpace(reportDigest),
	}, "\n")
}
