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
  qhtml refresh --lane-root <lane_root> [--source <original.html>] [--write]
  qhtml witness --lane-root <lane_root> --export <rendered.html> [--source <original.html>] [--write]
  qhtml visual-witness --export <rendered.html> [--console-report <console.json>] [--screenshot <screenshot.png>] [--viewport desktop|mobile] [--write]

Options:
  --project <root>      Project root. Defaults to current working directory.
  --state-root <root>   State root. Defaults to command-specific .qhtml folders.
`)
}
