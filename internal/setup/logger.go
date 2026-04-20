package setup

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Centralizes printing structured and formatted messages to stdout
type logger struct {
	indent  int
	verbose bool
	colors  struct {
		reset  string
		red    string
		green  string
		yellow string
		blue   string
	}
}

func newLogger(verbose bool) (new *logger) {
	new = &logger{verbose: verbose}

	// ANSI Color Codes
	if term.IsTerminal(int(os.Stdin.Fd())) {
		new.colors.reset = "\033[0m"
		new.colors.red = "\033[31m"
		new.colors.green = "\033[32m"
		new.colors.yellow = "\033[33m"
		new.colors.blue = "\033[34m"
	}
	return
}

func (logger *logger) prefix() string {
	return strings.Repeat("  ", logger.indent)
}

func (logger *logger) Step(name string) {
	fmt.Printf("%s==> %s\n", logger.prefix(), name)
}

func (logger *logger) Success(msg string, args ...any) {
	fmt.Printf("%s[+]%s %s%s\n",
		logger.colors.green, logger.colors.reset,
		logger.prefix(), fmt.Sprintf(msg, args...))
}

func (logger *logger) Info(msg string, args ...any) {
	fmt.Printf("%s[*]%s %s%s\n",
		logger.colors.blue, logger.colors.reset,
		logger.prefix(), fmt.Sprintf(msg, args...))
}

func (logger *logger) Error(msg string, args ...any) {
	fmt.Printf("%s[-]%s %s%s\n",
		logger.colors.red, logger.colors.reset,
		logger.prefix(), fmt.Sprintf(msg, args...))
}

func (logger *logger) Verbose(msg string, args ...any) {
	if logger.verbose {
		fmt.Printf("[*] %s%s\n", logger.prefix(), fmt.Sprintf(msg, args...))
	}
}

func (logger *logger) Indent() { logger.indent++ }
func (logger *logger) Dedent() { logger.indent-- }
