package qhtml

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RunnerProofVerificationSchemaVersion = "qhtml.runner_proof_verification.v1"

type VerifyRunnerProofRequest struct {
	ProjectRoot   string
	ProofPath     string
	PublicKey     string
	StateRoot     string
	WriteEvidence bool
	ObservedAt    time.Time
}

type RunnerProofVerificationReceipt struct {
	SchemaVersion   string            `json:"schema_version"`
	Status          string            `json:"status"`
	ProductID       string            `json:"product_id"`
	ProjectRoot     string            `json:"project_root"`
	ProofPath       string            `json:"proof_path"`
	StateRoot       string            `json:"state_root"`
	ReceiptPath     string            `json:"receipt_path,omitempty"`
	ProofDigest     string            `json:"proof_digest"`
	PublicKeyDigest string            `json:"public_key_digest"`
	Verified        bool              `json:"verified"`
	Algorithm       string            `json:"algorithm"`
	Checks          []string          `json:"checks"`
	NegativeCases   []string          `json:"negative_cases"`
	ManagedBy       string            `json:"managed_by"`
	Policy          string            `json:"policy"`
	ObservedAt      string            `json:"observed_at"`
	Details         map[string]string `json:"details,omitempty"`
}

func VerifyRunnerProof(req VerifyRunnerProofRequest) (RunnerProofVerificationReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return RunnerProofVerificationReceipt{}, err
	}
	if strings.TrimSpace(req.ProofPath) == "" {
		return RunnerProofVerificationReceipt{}, errors.New("qhtml verify-runner-proof --proof required")
	}
	publicKey, err := decodeKeyOrSignature(req.PublicKey)
	if err != nil {
		return RunnerProofVerificationReceipt{}, err
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return RunnerProofVerificationReceipt{}, errors.New("qhtml verify-runner-proof public key must be ed25519 32 bytes")
	}
	proofPath := absPath(projectRoot, req.ProofPath)
	proof, err := readRunnerProof(proofPath)
	if err != nil {
		return RunnerProofVerificationReceipt{}, err
	}
	signature, err := decodeKeyOrSignature(proof.Signature)
	if err != nil {
		return RunnerProofVerificationReceipt{}, err
	}
	if len(signature) != ed25519.SignatureSize {
		return RunnerProofVerificationReceipt{}, errors.New("qhtml verify-runner-proof signature must be ed25519 64 bytes")
	}
	payload := proof.SignaturePayload
	if strings.TrimSpace(payload) == "" {
		payload = runnerProofSignaturePayload(proof.RunnerID, proof.RunnerVersion, proof.ReportDigest)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(payload), signature) {
		return RunnerProofVerificationReceipt{}, errors.New("qhtml verify-runner-proof signature verification failed")
	}
	proofDigest, err := digestFile(proofPath)
	if err != nil {
		return RunnerProofVerificationReceipt{}, err
	}
	publicKeyDigest := sha256Hex(publicKey)
	stateRoot := runnerProofVerificationRoot(projectRoot, req.StateRoot)
	receipt := RunnerProofVerificationReceipt{
		SchemaVersion:   RunnerProofVerificationSchemaVersion,
		Status:          "runner_proof_verified",
		ProductID:       "qhtml",
		ProjectRoot:     slashClean(projectRoot),
		ProofPath:       slashClean(proofPath),
		StateRoot:       slashClean(stateRoot),
		ProofDigest:     proofDigest,
		PublicKeyDigest: publicKeyDigest,
		Verified:        true,
		Algorithm:       "ed25519",
		Checks: []string{
			"runner_proof_schema_validated",
			"public_key_size_validated",
			"signature_size_validated",
			"signature_payload_verified",
		},
		NegativeCases: []string{
			"missing_proof_rejected",
			"bad_public_key_rejected",
			"bad_signature_rejected",
			"wrong_schema_rejected",
		},
		ManagedBy:  "go_runner_proof_verifier",
		Policy:     "runner_signature_must_verify_against_ed25519_public_key_before_external_handoff_claims",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"verification_role": "turns signed runner proof claim into cryptographically verified runner proof receipt",
		},
	}
	if req.WriteEvidence {
		verificationDigest := sha256Hex([]byte(proofDigest + "\n" + publicKeyDigest))
		receiptPath := filepath.Join(stateRoot, verificationDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_runner_proof_verification.json")
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func readRunnerProof(path string) (RunnerProofReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunnerProofReceipt{}, err
	}
	var proof RunnerProofReceipt
	if err := json.Unmarshal(stripUTF8BOM(data), &proof); err != nil {
		return RunnerProofReceipt{}, err
	}
	if proof.SchemaVersion != RunnerProofSchemaVersion {
		return RunnerProofReceipt{}, errors.New("qhtml verify-runner-proof rejected wrong proof schema: " + proof.SchemaVersion)
	}
	if proof.Status != "runner_proof" {
		return RunnerProofReceipt{}, errors.New("qhtml verify-runner-proof rejected wrong proof status: " + proof.Status)
	}
	if proof.ProductID != "qhtml" {
		return RunnerProofReceipt{}, errors.New("qhtml verify-runner-proof rejected non-qhtml proof")
	}
	return proof, nil
}

func decodeKeyOrSignature(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty encoded key/signature")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return hex.DecodeString(value)
}

func runnerProofVerificationRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "runner_verifications")
}
