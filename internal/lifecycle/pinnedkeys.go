package lifecycle

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Finds running process via program name and config path and issues signal to reload pinned keys config file
func IssueLivePinnedKeyReload(configPath string, programName string) (err error) {
	programName = filepath.Base(programName)

	cmd := exec.Command("ps", "-axo", "pid,comm,args")
	out, err := cmdCombinedOutput(cmd)
	if err != nil {
		err = fmt.Errorf("process list retrieval failed: %w: %s", err, string(out))
		return
	}

	var foundPID int

	reader := bytes.NewReader(out)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Format: PID Comm Arg1 Arg2 Arg3
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pid := fields[0]
		prog := fields[1]
		args := strings.Join(fields[2:], " ")

		prog = strings.TrimSpace(prog)
		prog = strings.Trim(prog, "\n")

		if prog != programName {
			continue
		}

		if !strings.Contains(args, "--config "+configPath) &&
			!strings.Contains(args, "-c "+configPath) {
			continue
		}

		if foundPID != 0 {
			// Already found a process, not supported when multiple programs running
			err = fmt.Errorf("found multiple running processes matching name %q and arg list %q: refusing to reload due to ambiguity",
				programName, configPath)
			return
		}

		// Found matching running program - save and continue search
		foundPID, err = strconv.Atoi(pid)
		if err != nil {
			err = fmt.Errorf("failed converting pid %s to number: %w", pid, err)
			return
		}
	}

	if foundPID == 0 {
		// No-op - but tell user
		fmt.Printf("Could not find running process to send pinned keys reload signal (no error)\n")
		return
	}

	// Issue custom reload signal to tell process to pick up new file contents
	err = syscallKill(foundPID, PinKeyReloadSignal)
	if err != nil {
		err = fmt.Errorf("failed to send pinned keys reload signal: %w", err)
		return
	}

	return
}
