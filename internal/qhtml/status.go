package qhtml

import "time"

const ProductStatusSchemaVersion = "qhtml.product_status.v1"

type ProductStatusRequest struct {
	ObservedAt time.Time
}

type ProductStatus struct {
	SchemaVersion   string        `json:"schema_version"`
	Status          string        `json:"status"`
	ProductID       string        `json:"product_id"`
	Name            string        `json:"name"`
	Definition      string        `json:"definition"`
	RuntimeCommands []string      `json:"runtime_commands"`
	Implemented     []ProductItem `json:"implemented"`
	Gaps            []ProductItem `json:"gaps"`
	Percent         int           `json:"percent"`
	Policy          string        `json:"policy"`
	ObservedAt      string        `json:"observed_at"`
}

type ProductItem struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func Status(req ProductStatusRequest) ProductStatus {
	if req.ObservedAt.IsZero() {
		req.ObservedAt = time.Now()
	}
	implemented := []ProductItem{
		{ID: "go_change_manager", Status: "implemented", Reason: "lane/source digest refresh exists"},
		{ID: "json_cli_surface", Status: "implemented", Reason: "status and refresh commands emit machine-readable JSON"},
		{ID: "receipt_state_store", Status: "implemented", Reason: ".qhtml managed state and receipt paths are deterministic"},
		{ID: "standalone_module", Status: "implemented", Reason: "no NeuronFS runtime dependency"},
	}
	gaps := []ProductItem{
		{ID: "html_projection_renderer", Status: "missing", Reason: "standalone renderer is not yet extracted"},
		{ID: "media_slot_resolver", Status: "missing", Reason: "media slot language is specified but not implemented in standalone"},
		{ID: "vorq_render_witness", Status: "missing", Reason: "render receipt seal is not implemented"},
		{ID: "browser_visual_witness", Status: "missing", Reason: "no screenshot/console/responsive witness yet"},
		{ID: "targeting_tombstone", Status: "missing", Reason: "cell/slot target and rollback commands are not implemented"},
		{ID: "bidirectional_sync", Status: "missing", Reason: "export changes do not yet become lane patch proposals"},
	}
	total := len(implemented) + len(gaps)
	percent := 0
	if total > 0 {
		percent = len(implemented) * 100 / total
	}
	return ProductStatus{
		SchemaVersion: ProductStatusSchemaVersion,
		Status:        "standalone_seed",
		ProductID:     "qhtml",
		Name:          "QHTML",
		Definition:    "QHTML is a folder-native render contract where generated HTML is disposable output and folder lane state is source truth.",
		RuntimeCommands: []string{
			"qhtml status",
			"qhtml refresh --lane-root <lane_root> [--source <original.html>] [--write]",
		},
		Implemented: implemented,
		Gaps:        gaps,
		Percent:     percent,
		Policy:      "folder_lane_is_source_truth; html_is_projection; go_digest_refresh_is_correctness_layer",
		ObservedAt:  req.ObservedAt.UTC().Format(time.RFC3339),
	}
}
