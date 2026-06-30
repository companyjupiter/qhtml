# QHTML

QHTML is a folder-native render contract for reducing repeated full HTML scans and targeting exact UI parts by folder address.

It does not treat `index.html` as source truth. The source is a managed folder lane:

```text
opaque folder rune lane
  + optional original/source digest
  + media slot placement
  + render policy
  + witness receipts
  -> disposable HTML projection
```

HTML is output cache. The folder is the address system. QHTML exists so UI artifacts can be edited, audited, regenerated, and handed off without trusting an active editor tab or a single generated HTML file.

The core value is practical:

- reduce repeated full HTML scans by checking lane/source digests first
- target exact UI cells, media slots, rollback points, and future patch proposals by folder path
- keep generated HTML disposable while preserving a stable source lane

## Install

```powershell
go install github.com/companyjupiter/qhtml/cmd/qhtml@latest
```

Local development:

```powershell
go test ./...
go run ./cmd/qhtml status
```

## Commands

```powershell
qhtml status
qhtml refresh --lane-root <lane_root> [--source <original.html>] [--write]
qhtml witness --lane-root <lane_root> --export <rendered.html> [--source <original.html>] [--write]
qhtml visual-witness --export <rendered.html> [--console-report <console.json>] [--screenshot <screenshot.png>] [--viewport desktop|mobile] [--write]
qhtml layout-witness --export <rendered.html> --report <layout-report.json> [--write]
qhtml target --lane-root <lane_root> --path <lane_relative_target> [--kind cell|media|style|event] [--write]
qhtml tombstone --lane-root <lane_root> --path <lane_relative_target> [--reason <why>] [--write]
qhtml rollback --lane-root <lane_root> --path <lane_relative_target> --to-digest <digest> [--source-receipt <receipt>] [--write]
qhtml import-proposal --lane-root <lane_root> --export <rendered.html> [--path <lane_relative_target>] [--source-receipt <receipt>] [--write]
```

`refresh` computes a stable digest over the lane folder and optional source file, compares it with the previous state, and reports:

- `lane_changed`
- `source_changed`
- `needs_render_refresh`
- `state_path`
- `receipt_path` when `--write` is used

State is stored under:

```text
.qhtml/managed/<lane-key>/state.json
.qhtml/managed/<lane-key>/receipts/*.qhtml_refresh.json
.qhtml/witnesses/<render-key>/*.qhtml_witness.json
.qhtml/visual_witnesses/<visual-key>/*.qhtml_visual_witness.json
.qhtml/layout_witnesses/<layout-key>/*.qhtml_layout_witness.json
.qhtml/targets/<target-key>/targets/*.qhtml_target.json
.qhtml/targets/<target-key>/tombstones/*.qhtml_tombstone.json
.qhtml/targets/<target-key>/rollbacks/*.qhtml_rollback.json
.qhtml/import_proposals/<proposal-key>/*.qhtml_import_proposal.json
```

The manager ignores its own runtime artifacts while hashing a lane:

- `.qhtml/`
- `.git/`
- `dist/`
- the configured `--state-root` if it is inside the lane

This prevents the classic self-contamination failure where writing a state file causes every later refresh to report a false change.

## Why It Is Separate

QHTML started inside NeuronFS, but it is a product boundary of its own:

- folder-native UI source management
- deterministic change detection
- disposable HTML export philosophy
- future browser/Vorq witness layer
- cross-platform adapter surface

NeuronFS can embed QHTML, but QHTML must be usable without NeuronFS.

## Current Level

Implemented:

- Go-native lane/source digest manager
- HTML fullscan reduction through digest-first refresh
- seed precision targeting surface through stable folder lane addresses
- JSON status and refresh CLI
- receipt writing for refresh events
- deterministic directory hashing
- exclusive refresh lock
- symlink target hashing without following links
- render export witness receipts binding lane/source/export digests
- browser visual artifact witness receipts for nonblank export, zero console errors, and optional screenshot digest
- browser layout witness receipts for viewport nonblank, console, and overflow evidence
- target/tombstone/rollback receipts for lane-relative cell/media/style/event addresses
- import proposal receipts that turn export changes into lane patch proposals without overwriting source folders
- tests for initial state, no-change state, source change, and lane change

Not complete:

- HTML projection renderer
- media slot resolver
- Vorq render receipt

## Blind Spots Already Simulated

- State directory inside the lane: fixed by excluding `.qhtml` and `--state-root` from lane digest.
- File deletion: covered by digest tests.
- Atomic state writes: state and receipt JSON are written via temp file then rename.
- Concurrent refresh: guarded by an exclusive lock file.
- Symlink drift: symlinks are hashed by link target path and are not followed outside the lane.
- Watcher loss: correctness does not depend on a long-running watcher; polling `refresh` is the source of truth.
- Blank export: `visual-witness` rejects HTML without visible text.
- Console errors: `visual-witness` rejects console reports containing error entries.
- Empty screenshot: `visual-witness` rejects zero-byte screenshot artifacts.
- Layout report drift: `layout-witness` rejects blank viewports, console errors, overflow, and invalid viewport dimensions.
- Target escape: `target` rejects lane-relative paths that escape the lane root.
- Destructive targeting: `tombstone` and `rollback` write receipts/proposals first and do not mutate the lane without an external promotion gate.
- Import mutation: `import-proposal` writes a proposal receipt and leaves the lane target digest unchanged.
- Import escape: `import-proposal` rejects target paths that escape the lane root.

Remaining blind spots:

- Large binary media folders need a future size budget and chunked hashing.
- Vorq receipts, signed browser runner proof, and media resolver are still outside the standalone seed.

## Potential Assessment

QHTML has high product potential if it stays focused on one claim:

> A UI artifact should have a folder-addressable source of truth, not only a generated HTML file.

Strongest markets:

- AI-generated UI source control
- precision UI targeting without full HTML rescans
- design handoff with receipts
- visual QA and browser witness automation
- NeuronFS or agent-runtime UI artifact lanes
- cross-platform local-first site/app builders

Current potential score from `qhtml status`: `82/100`.

That is not a maturity score. It means the core product thesis is strong, while the implementation is still a seed. The next milestones are:

1. Extract standalone `render-folder`.
2. Add automated browser layout witness.
3. Add Vorq-compatible render receipt.
4. Add signed browser runner proof.
5. Add media size budgets and chunked hashing.

## Product Rule

QHTML never promotes generated HTML to source truth.

If a folder or original source changes, `qhtml refresh --write` must make the change visible before render, witness, or promotion.
