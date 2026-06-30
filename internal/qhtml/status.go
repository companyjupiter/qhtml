package qhtml

import "time"

const ProductStatusSchemaVersion = "qhtml.product_status.v1"

type ProductStatusRequest struct {
	ObservedAt time.Time
}

type ProductStatus struct {
	SchemaVersion    string        `json:"schema_version"`
	Status           string        `json:"status"`
	ProductID        string        `json:"product_id"`
	Name             string        `json:"name"`
	Definition       string        `json:"definition"`
	ValueProposition []string      `json:"value_proposition"`
	RuntimeCommands  []string      `json:"runtime_commands"`
	Implemented      []ProductItem `json:"implemented"`
	Gaps             []ProductItem `json:"gaps"`
	Potential        []ProductItem `json:"potential"`
	PotentialScore   int           `json:"potential_score"`
	NextMilestones   []string      `json:"next_milestones"`
	Percent          int           `json:"percent"`
	Policy           string        `json:"policy"`
	ObservedAt       string        `json:"observed_at"`
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
		{ID: "process_lock", Status: "implemented", Reason: "refresh writes are guarded by an exclusive lock file"},
		{ID: "symlink_policy", Status: "implemented", Reason: "symlinks are hashed as link targets and are not followed"},
		{ID: "html_fullscan_reduction", Status: "implemented", Reason: "refresh compares folder/source digests so callers avoid rescanning full HTML for unchanged artifacts"},
		{ID: "render_export_witness", Status: "implemented", Reason: "witness binds lane/source/export digests into .qhtml/witnesses receipts"},
		{ID: "browser_visual_artifact_witness", Status: "implemented", Reason: "visual-witness seals nonblank export, zero console errors, and optional screenshot digest"},
		{ID: "browser_layout_witness", Status: "implemented", Reason: "layout-witness seals viewport nonblank, console, and overflow evidence from an external browser runner"},
		{ID: "precision_targeting_surface", Status: "implemented", Reason: "target command resolves lane-relative cell/media addresses and seals target digests"},
		{ID: "targeting_tombstone", Status: "implemented", Reason: "target/tombstone/rollback commands create receipt-first target, tombstone, and rollback proposals"},
		{ID: "bidirectional_sync", Status: "implemented", Reason: "import-proposal turns export changes into lane patch proposal receipts without overwriting source lanes"},
	}
	gaps := []ProductItem{
		{ID: "html_projection_renderer", Status: "missing", Reason: "standalone renderer is not yet extracted"},
		{ID: "media_slot_resolver", Status: "missing", Reason: "media slot language is specified but not implemented in standalone"},
		{ID: "vorq_render_witness", Status: "missing", Reason: "render receipt seal is not implemented"},
	}
	potential := []ProductItem{
		{ID: "ai_ui_source_control", Status: "high", Reason: "folder lane makes AI-generated UI auditable without repeated full HTML scans"},
		{ID: "design_handoff", Status: "high", Reason: "render receipts and browser witness can become a concrete handoff contract"},
		{ID: "precision_ui_targeting", Status: "high", Reason: "stable folder addresses can target exact cells, slots, media, and rollback points"},
		{ID: "cross_platform_adapter", Status: "medium_high", Reason: "digest manager is already platform-neutral Go; render adapters remain"},
		{ID: "neuronfs_embedding", Status: "high", Reason: "NeuronFS can use QHTML as a UI artifact lane without owning the product"},
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
		Definition:    "QHTML is a folder-native render contract that reduces repeated full HTML scans and enables precise UI targeting through folder lane addresses.",
		ValueProposition: []string{
			"reduce repeated full HTML scans by comparing lane/source digests first",
			"target exact UI cells, media slots, rollback points, and future patch proposals by folder address",
			"treat generated HTML as disposable projection, not source truth",
			"make AI-generated UI artifacts auditable before render or promotion",
		},
		RuntimeCommands: []string{
			"qhtml status",
			"qhtml refresh --lane-root <lane_root> [--source <original.html>] [--write]",
			"qhtml witness --lane-root <lane_root> --export <rendered.html> [--source <original.html>] [--write]",
			"qhtml visual-witness --export <rendered.html> [--console-report <console.json>] [--screenshot <screenshot.png>] [--write]",
			"qhtml layout-witness --export <rendered.html> --report <layout-report.json> [--write]",
			"qhtml target --lane-root <lane_root> --path <lane_relative_target> [--write]",
			"qhtml tombstone --lane-root <lane_root> --path <lane_relative_target> [--reason <why>] [--write]",
			"qhtml rollback --lane-root <lane_root> --path <lane_relative_target> --to-digest <digest> [--write]",
			"qhtml import-proposal --lane-root <lane_root> --export <rendered.html> [--path <lane_relative_target>] [--write]",
		},
		Implemented:    implemented,
		Gaps:           gaps,
		Potential:      potential,
		PotentialScore: 82,
		NextMilestones: []string{
			"extract standalone render-folder",
			"add Vorq-compatible render receipt",
			"add signed browser runner proof",
		},
		Percent:    percent,
		Policy:     "folder_lane_is_source_truth; html_is_projection; go_digest_refresh_is_correctness_layer",
		ObservedAt: req.ObservedAt.UTC().Format(time.RFC3339),
	}
}
