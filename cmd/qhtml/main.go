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

Options:
  --project <root>      Project root. Defaults to current working directory.
  --state-root <root>   Managed state root. Defaults to .qhtml/managed.
`)
}
