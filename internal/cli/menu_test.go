package cli

import (
	"bytes"
	"flag"
	"os"
	"sdsyslog/internal/global"
	"strings"
	"testing"
)

func TestPrintHelpMenu_RootOutput(t *testing.T) {
	// Make output deterministic
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{global.ProgBaseName}

	root := DefineOptions()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = SetGlobalArguments(fs)

	// Capture stdout
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintHelpMenu(fs, "", root)

	w.Close()
	os.Stdout = origStdout
	buf.ReadFrom(r)

	got := buf.String()
	got = strings.ReplaceAll(got, "\r\n", "\n")

	expected := `Usage: ` + global.ProgBaseName + ` [subcommand]

` + root.Description + `
` + root.FullDescription + `

  Subcommands:
    configure   - Setup Actions
    ` + global.RecvMode + `     - Receive Messages
    ` + global.SendMode + `        - Send Messages
    version     - Show Version Information

  Options:
  -v, --verbosity  Increase detailed progress messages (Higher is more verbose) <0...5> [default: 1]
` + helpMenuTrailer

	expected = strings.ReplaceAll(expected, "\r\n", "\n")

	if got != expected {
		t.Fatalf("unexpected help output\n\nGOT:\n%s\n\nWANT:\n%s", got, expected)
	}
}
