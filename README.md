# QHTML

QHTML is a folder-native render contract.

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
```

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
- JSON status and refresh CLI
- receipt writing for refresh events
- deterministic directory hashing
- tests for initial state, no-change state, source change, and lane change

Not complete:

- HTML projection renderer
- media slot resolver
- browser visual witness
- Vorq render receipt
- target/tombstone/rollback commands
- bidirectional sync from export changes back to lane patch proposals

## Product Rule

QHTML never promotes generated HTML to source truth.

If a folder or original source changes, `qhtml refresh --write` must make the change visible before render, witness, or promotion.
