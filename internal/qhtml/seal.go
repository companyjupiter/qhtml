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

const SealSchemaVersion = "qhtml.vorq_seal.v1"

type SealRequest struct {
	ProjectRoot        string
	WitnessPath        string
	ImportProposalPath string
	VisualWitnessPath  string
	LayoutWitnessPath  string
	RunnerProofPath    string
	StateRoot          string
	WriteEvidence      bool
	ObservedAt         time.Time
}

type SealReceipt struct {
	SchemaVersion  string            `json:"schema_version"`
	Status         string            `json:"status"`
	ProductID      string            `json:"product_id"`
	ProjectRoot    string            `json:"project_root"`
	StateRoot      string            `json:"state_root"`
	ReceiptPath    string            `json:"receipt_path,omitempty"`
	InputReceipts  map[string]string `json:"input_receipts"`
	InputDigests   map[string]string `json:"input_digests"`
	SealDigest     string            `json:"seal_digest"`
	PromotionReady bool              `json:"promotion_ready"`
	Checks         []string          `json:"checks"`
	NegativeCases  []string          `json:"negative_cases"`
	ManagedBy      string            `json:"managed_by"`
	Policy         string            `json:"policy"`
	ObservedAt     string            `json:"observed_at"`
	Details        map[string]string `json:"details,omitempty"`
}

type receiptHeader struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	ProductID     string `json:"product_id"`
}

func Seal(req SealRequest) (SealReceipt, error) {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	projectRoot, err := projectRoot(req.ProjectRoot)
	if err != nil {
		return SealReceipt{}, err
	}
	inputs, err := sealInputs(projectRoot, req)
	if err != nil {
		return SealReceipt{}, err
	}
	digests := map[string]string{}
	for role, path := range inputs {
		digest, digestErr := digestFile(path)
		if digestErr != nil {
			return SealReceipt{}, digestErr
		}
		digests[role] = digest
	}
	sealDigest := sealDigest(digests)
	stateRoot := sealRoot(projectRoot, req.StateRoot)
	receipt := SealReceipt{
		SchemaVersion:  SealSchemaVersion,
		Status:         "vorq_seal",
		ProductID:      "qhtml",
		ProjectRoot:    slashClean(projectRoot),
		StateRoot:      slashClean(stateRoot),
		InputReceipts:  slashMap(inputs),
		InputDigests:   digests,
		SealDigest:     sealDigest,
		PromotionReady: true,
		Checks: []string{
			"render_witness_receipt_required",
			"input_receipt_schema_validated",
			"input_receipt_digest_bound",
			"seal_digest_stable_ordered",
		},
		NegativeCases: []string{
			"missing_witness_rejected",
			"wrong_schema_rejected",
			"wrong_status_rejected",
		},
		ManagedBy:  "go_vorq_seal",
		Policy:     "vorq_compatible_receipt_combiner; no_lane_mutation; promotion_requires_bound_input_receipts",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
		Details: map[string]string{
			"seal_role": "combines QHTML witness/import/layout/visual receipts into a final promotion receipt",
		},
	}
	if req.WriteEvidence {
		receiptPath := filepath.Join(stateRoot, sealDigest[:16], req.ObservedAt.UTC().Format("20060102T150405Z")+".qhtml_seal.json")
		if err := writeJSON(receiptPath, receipt); err != nil {
			return receipt, err
		}
		receipt.ReceiptPath = slashClean(receiptPath)
	}
	return receipt, nil
}

func sealInputs(projectRoot string, req SealRequest) (map[string]string, error) {
	inputs := map[string]string{}
	if strings.TrimSpace(req.WitnessPath) == "" {
		return nil, errors.New("qhtml seal --witness required")
	}
	witnessPath := absPath(projectRoot, req.WitnessPath)
	if err := validateReceipt(witnessPath, WitnessSchemaVersion, "render_witness"); err != nil {
		return nil, err
	}
	inputs["witness"] = witnessPath
	optional := []struct {
		role   string
		path   string
		schema string
		status string
	}{
		{"import_proposal", req.ImportProposalPath, ImportProposalSchemaVersion, "import_proposal"},
		{"visual_witness", req.VisualWitnessPath, VisualWitnessSchemaVersion, "visual_witness"},
		{"layout_witness", req.LayoutWitnessPath, LayoutWitnessSchemaVersion, "layout_witness"},
		{"runner_proof", req.RunnerProofPath, RunnerProofSchemaVersion, "runner_proof"},
	}
	for _, item := range optional {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		path := absPath(projectRoot, item.path)
		if err := validateReceipt(path, item.schema, item.status); err != nil {
			return nil, err
		}
		inputs[item.role] = path
	}
	return inputs, nil
}

func validateReceipt(path, schema, status string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var header receiptHeader
	if err := json.Unmarshal(stripUTF8BOM(data), &header); err != nil {
		return err
	}
	if header.SchemaVersion != schema {
		return errors.New("qhtml seal rejected wrong receipt schema: " + header.SchemaVersion)
	}
	if header.Status != status {
		return errors.New("qhtml seal rejected wrong receipt status: " + header.Status)
	}
	if header.ProductID != "qhtml" {
		return errors.New("qhtml seal rejected non-qhtml receipt")
	}
	return nil
}

func sealDigest(digests map[string]string) string {
	keys := make([]string, 0, len(digests))
	for key := range digests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{SealSchemaVersion}
	for _, key := range keys {
		parts = append(parts, key+"="+digests[key])
	}
	return sha256Hex([]byte(strings.Join(parts, "\n")))
}

func sealRoot(projectRoot, stateRoot string) string {
	if strings.TrimSpace(stateRoot) != "" {
		return absPath(projectRoot, stateRoot)
	}
	return filepath.Join(projectRoot, ".qhtml", "seals")
}

func slashMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = slashClean(value)
	}
	return out
}
