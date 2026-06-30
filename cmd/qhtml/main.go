package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/companyjupiter/qhtml/internal/qhtml"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	command := "status"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "status", "product", "readiness":
		return encode(qhtml.Status(qhtml.ProductStatusRequest{}), nil)
	case "render-folder":
		fs := flag.NewFlagSet("render-folder", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		outPath := fs.String("out", "", "rendered HTML output path")
		title := fs.String("title", "", "optional HTML title")
		stateRoot := fs.String("state-root", "", "optional render state root; default .qhtml/renders")
		write := fs.Bool("write", false, "write HTML projection and render receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *laneRoot == "" && fs.NArg() > 0 {
			*laneRoot = fs.Arg(0)
		}
		result, err := qhtml.RenderFolder(qhtml.RenderFolderRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			ExportPath:    *outPath,
			Title:         *title,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "resolve-media":
		fs := flag.NewFlagSet("resolve-media", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		slotRoot := fs.String("slot-root", "", "lane-relative media slot root; default 04")
		outDir := fs.String("out-dir", "", "optional media export copy directory")
		stateRoot := fs.String("state-root", "", "optional media state root; default .qhtml/media")
		maxBytes := fs.Int64("max-bytes", 0, "per-asset max bytes; default 25MiB")
		write := fs.Bool("write", false, "write media receipt and optional export copies")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *laneRoot == "" && fs.NArg() > 0 {
			*laneRoot = fs.Arg(0)
		}
		result, err := qhtml.ResolveMedia(qhtml.MediaRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			SlotRoot:      *slotRoot,
			OutDir:        *outDir,
			StateRoot:     *stateRoot,
			MaxBytes:      *maxBytes,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "adapter-conformance":
		fs := flag.NewFlagSet("adapter-conformance", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		stateRoot := fs.String("state-root", "", "optional adapter conformance state root; default .qhtml/adapter_conformance")
		write := fs.Bool("write", false, "write adapter conformance receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *laneRoot == "" && fs.NArg() > 0 {
			*laneRoot = fs.Arg(0)
		}
		result, err := qhtml.AdapterConformance(qhtml.AdapterConformanceRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "refresh", "manage", "watch":
		fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		sourcePath := fs.String("source", "", "optional original/source file")
		stateRoot := fs.String("state-root", "", "optional managed state root; default .qhtml/managed")
		write := fs.Bool("write", false, "write managed state and receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *laneRoot == "" && fs.NArg() > 0 {
			*laneRoot = fs.Arg(0)
		}
		result, err := qhtml.Manage(qhtml.ManageRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			SourcePath:    *sourcePath,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "witness", "render-witness":
		fs := flag.NewFlagSet("witness", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		sourcePath := fs.String("source", "", "optional original/source file")
		exportPath := fs.String("export", "", "rendered/exported HTML file")
		stateRoot := fs.String("state-root", "", "optional witness state root; default .qhtml/witnesses")
		write := fs.Bool("write", false, "write witness receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *laneRoot == "" && fs.NArg() > 0 {
			*laneRoot = fs.Arg(0)
		}
		result, err := qhtml.Witness(qhtml.WitnessRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			SourcePath:    *sourcePath,
			ExportPath:    *exportPath,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "visual-witness":
		fs := flag.NewFlagSet("visual-witness", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		exportPath := fs.String("export", "", "rendered/exported HTML file")
		consoleReport := fs.String("console-report", "", "optional browser console report JSON/text")
		screenshot := fs.String("screenshot", "", "optional browser screenshot file")
		viewport := fs.String("viewport", "", "optional viewport label")
		stateRoot := fs.String("state-root", "", "optional visual witness state root; default .qhtml/visual_witnesses")
		write := fs.Bool("write", false, "write visual witness receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.VisualWitness(qhtml.VisualWitnessRequest{
			ProjectRoot:       *projectRoot,
			ExportPath:        *exportPath,
			ConsoleReportPath: *consoleReport,
			ScreenshotPath:    *screenshot,
			Viewport:          *viewport,
			StateRoot:         *stateRoot,
			WriteEvidence:     *write,
		})
		return encode(result, err)
	case "layout-witness":
		fs := flag.NewFlagSet("layout-witness", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		exportPath := fs.String("export", "", "rendered/exported HTML file")
		reportPath := fs.String("report", "", "browser layout report JSON")
		stateRoot := fs.String("state-root", "", "optional layout witness state root; default .qhtml/layout_witnesses")
		write := fs.Bool("write", false, "write layout witness receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.LayoutWitness(qhtml.LayoutWitnessRequest{
			ProjectRoot:   *projectRoot,
			ExportPath:    *exportPath,
			ReportPath:    *reportPath,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "target":
		fs := flag.NewFlagSet("target", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		targetPath := fs.String("path", "", "lane-relative target path")
		kind := fs.String("kind", "", "target kind; default cell")
		stateRoot := fs.String("state-root", "", "optional target state root; default .qhtml/targets")
		write := fs.Bool("write", false, "write target receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.Target(qhtml.TargetRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			TargetPath:    *targetPath,
			Kind:          *kind,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "tombstone":
		fs := flag.NewFlagSet("tombstone", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		targetPath := fs.String("path", "", "lane-relative target path")
		kind := fs.String("kind", "", "target kind; default cell")
		reason := fs.String("reason", "", "tombstone reason")
		stateRoot := fs.String("state-root", "", "optional target state root; default .qhtml/targets")
		write := fs.Bool("write", false, "write tombstone receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.Tombstone(qhtml.TombstoneRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			TargetPath:    *targetPath,
			Kind:          *kind,
			Reason:        *reason,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "rollback":
		fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		targetPath := fs.String("path", "", "lane-relative target path")
		kind := fs.String("kind", "", "target kind; default cell")
		toDigest := fs.String("to-digest", "", "target digest to roll back to")
		sourceReceipt := fs.String("source-receipt", "", "optional source receipt path")
		stateRoot := fs.String("state-root", "", "optional target state root; default .qhtml/targets")
		write := fs.Bool("write", false, "write rollback proposal receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.Rollback(qhtml.RollbackRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			TargetPath:    *targetPath,
			Kind:          *kind,
			ToDigest:      *toDigest,
			SourceReceipt: *sourceReceipt,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "import-proposal":
		fs := flag.NewFlagSet("import-proposal", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		laneRoot := fs.String("lane-root", "", "QHTML lane root")
		exportPath := fs.String("export", "", "rendered/exported HTML file")
		targetPath := fs.String("path", "", "optional lane-relative target path")
		kind := fs.String("kind", "", "target kind; default cell when --path is used")
		sourceReceipt := fs.String("source-receipt", "", "optional source witness/target receipt path")
		stateRoot := fs.String("state-root", "", "optional import proposal state root; default .qhtml/import_proposals")
		write := fs.Bool("write", false, "write import proposal receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.ImportProposal(qhtml.ImportProposalRequest{
			ProjectRoot:   *projectRoot,
			LaneRoot:      *laneRoot,
			ExportPath:    *exportPath,
			TargetPath:    *targetPath,
			Kind:          *kind,
			SourceReceipt: *sourceReceipt,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "seal":
		fs := flag.NewFlagSet("seal", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		witnessPath := fs.String("witness", "", "render witness receipt path")
		importProposalPath := fs.String("import-proposal", "", "optional import proposal receipt path")
		visualWitnessPath := fs.String("visual-witness", "", "optional visual witness receipt path")
		layoutWitnessPath := fs.String("layout-witness", "", "optional layout witness receipt path")
		runnerProofPath := fs.String("runner-proof", "", "optional signed runner proof receipt path")
		runnerVerificationPath := fs.String("runner-verification", "", "optional runner proof verification receipt path")
		stateRoot := fs.String("state-root", "", "optional seal state root; default .qhtml/seals")
		write := fs.Bool("write", false, "write seal receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.Seal(qhtml.SealRequest{
			ProjectRoot:            *projectRoot,
			WitnessPath:            *witnessPath,
			ImportProposalPath:     *importProposalPath,
			VisualWitnessPath:      *visualWitnessPath,
			LayoutWitnessPath:      *layoutWitnessPath,
			RunnerProofPath:        *runnerProofPath,
			RunnerVerificationPath: *runnerVerificationPath,
			StateRoot:              *stateRoot,
			WriteEvidence:          *write,
		})
		return encode(result, err)
	case "runner-proof":
		fs := flag.NewFlagSet("runner-proof", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		reportPath := fs.String("report", "", "browser runner report JSON")
		runnerID := fs.String("runner-id", "", "browser runner identifier")
		runnerVersion := fs.String("runner-version", "", "browser runner version")
		signature := fs.String("signature", "", "runner signature claim")
		stateRoot := fs.String("state-root", "", "optional runner proof state root; default .qhtml/runner_proofs")
		write := fs.Bool("write", false, "write runner proof receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.RunnerProof(qhtml.RunnerProofRequest{
			ProjectRoot:   *projectRoot,
			ReportPath:    *reportPath,
			RunnerID:      *runnerID,
			RunnerVersion: *runnerVersion,
			Signature:     *signature,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "verify-runner-proof":
		fs := flag.NewFlagSet("verify-runner-proof", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		projectRoot := fs.String("project", "", "project root; default current working directory")
		proofPath := fs.String("proof", "", "runner proof receipt path")
		publicKey := fs.String("public-key", "", "ed25519 public key in base64 or hex")
		stateRoot := fs.String("state-root", "", "optional verification state root; default .qhtml/runner_verifications")
		write := fs.Bool("write", false, "write runner proof verification receipt")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		result, err := qhtml.VerifyRunnerProof(qhtml.VerifyRunnerProofRequest{
			ProjectRoot:   *projectRoot,
			ProofPath:     *proofPath,
			PublicKey:     *publicKey,
			StateRoot:     *stateRoot,
			WriteEvidence: *write,
		})
		return encode(result, err)
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "qhtml: unknown command %q\n", command)
		usage()
		return 2
	}
}

func encode(value any, err error) int {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "qhtml: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(value); encErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "qhtml: %v\n", encErr)
		return 1
	}
	return 0
}

func usage() {
	_, _ = fmt.Fprint(os.Stderr, `Usage:
  qhtml status
  qhtml render-folder --lane-root <lane_root> --out <rendered.html> [--title <title>] [--write]
  qhtml resolve-media --lane-root <lane_root> [--slot-root 04] [--out-dir <media_export_dir>] [--max-bytes <bytes>] [--write]
  qhtml adapter-conformance --lane-root <lane_root> [--write]
  qhtml refresh --lane-root <lane_root> [--source <original.html>] [--write]
  qhtml witness --lane-root <lane_root> --export <rendered.html> [--source <original.html>] [--write]
  qhtml visual-witness --export <rendered.html> [--console-report <console.json>] [--screenshot <screenshot.png>] [--viewport desktop|mobile] [--write]
  qhtml layout-witness --export <rendered.html> --report <layout-report.json> [--write]
  qhtml target --lane-root <lane_root> --path <lane_relative_target> [--kind cell|media|style|event] [--write]
  qhtml tombstone --lane-root <lane_root> --path <lane_relative_target> [--reason <why>] [--write]
  qhtml rollback --lane-root <lane_root> --path <lane_relative_target> --to-digest <digest> [--source-receipt <receipt>] [--write]
  qhtml import-proposal --lane-root <lane_root> --export <rendered.html> [--path <lane_relative_target>] [--source-receipt <receipt>] [--write]
  qhtml runner-proof --report <runner_report.json> --runner-id <id> --runner-version <version> --signature <signature> [--write]
  qhtml verify-runner-proof --proof <runner_proof_receipt> --public-key <ed25519_public_key> [--write]
  qhtml seal --witness <witness_receipt> [--import-proposal <proposal_receipt>] [--visual-witness <visual_receipt>] [--layout-witness <layout_receipt>] [--runner-proof <proof_receipt>] [--runner-verification <verification_receipt>] [--write]

Options:
  --project <root>      Project root. Defaults to current working directory.
  --state-root <root>   State root. Defaults to command-specific .qhtml folders.
`)
}
