package lifecycle

import (
	"os"
	"os/exec"
	"os/signal"

	"golang.org/x/sys/unix"
)

// Mockable functions that access external resources (low level OS interactions)

// Causes OS to relay incoming (filtering by provided signals) signals into sigChan.
var getSigNotifyChannel func(bufSize int, sig ...os.Signal) (sigChan chan os.Signal) = getSigNotifyChannelReal

func getSigNotifyChannelReal(bufSize int, sig ...os.Signal) (sigChan chan os.Signal) {
	sigChan = make(chan os.Signal, bufSize)
	signal.Notify(sigChan, sig...)
	return
}

// Send signal to process ID
var syscallKill func(pid int, sendSig unix.Signal) (err error) = syscallKillReal

func syscallKillReal(pid int, sendSig unix.Signal) (err error) {
	err = unix.Kill(pid, sendSig)
	return
}

// Wait for process to change state to the wstatus
var syscallWait4 func(pid int, wstatus *unix.WaitStatus, options int, rusage *unix.Rusage) (wpid int, err error) = syscallWait4Real

func syscallWait4Real(pid int, wstatus *unix.WaitStatus, options int, rusage *unix.Rusage) (wpid int, err error) {
	wpid, err = unix.Wait4(pid, wstatus, options, rusage)
	return
}

// Execute new binary replacing current running program (execve(2) system call)
var syscallExec func(argv0 string, argv []string, envv []string) (err error) = syscallExecReal

func syscallExecReal(argv0 string, argv []string, envv []string) (err error) {
	err = unix.Exec(argv0, argv, envv)
	return
}

// Executable returns the path name for the executable that started the current process
var osExecutable func() (exePath string, err error) = osExecReal

func osExecReal() (exePath string, err error) {
	exePath, err = os.Executable()
	return
}

// Runs a run-once command and returns the output (Stdout in out, stderr in err)
var cmdCombinedOutput func(cmd *exec.Cmd) (out []byte, err error) = cmdCombinedOutputReal

func cmdCombinedOutputReal(cmd *exec.Cmd) (out []byte, err error) {
	out, err = cmd.CombinedOutput()
	return
}

// Start starts the specified command but does not wait for it to complete
var cmdStart func(cmd *exec.Cmd) (err error) = cmdStartReal

func cmdStartReal(cmd *exec.Cmd) (err error) {
	err = cmd.Start()
	return
}

// Pipe returns a connected pair of Files; reads from reader return bytes written to writer.
var osPipe func() (reader *os.File, writer *os.File, err error) = osPipeReal

func osPipeReal() (reader *os.File, writer *os.File, err error) {
	reader, writer, err = os.Pipe()
	return
}

// Signal sends a signal to the Process
var cmdProcSignal func(cmd *exec.Cmd, sig os.Signal) (err error) = cmdProcSignalReal

func cmdProcSignalReal(cmd *exec.Cmd, sig os.Signal) (err error) {
	err = cmd.Process.Signal(sig)
	return
}

// Kill causes the Process to exit immediately. Kill does not wait until the Process has actually exited
var cmdProcKill func(cmd *exec.Cmd) (err error) = cmdProcKillReal

func cmdProcKillReal(cmd *exec.Cmd) (err error) {
	err = cmd.Process.Kill()
	return
}
