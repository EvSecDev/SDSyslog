package lifecycle

import (
	"os/exec"
	"sdsyslog/internal/global"
	"strings"
	"syscall"
	"testing"
)

func TestIssueLiveSigningKeyReload(t *testing.T) {
	type testCase struct {
		name                string
		configPath          string
		programName         string
		psOutput            string
		expectSyscall       bool
		expectSyscallTgtPID int
		expectedErr         string
	}

	tests := []testCase{
		{
			name:        "valid found program",
			configPath:  global.DefaultConfigRecv,
			programName: global.ProgBaseName,
			psOutput: `    PID COMMAND         COMMAND
      1 systemd         /sbin/init
     27 ksoftirqd/2     [ksoftirqd/2]
     28 kworker/2:0-cgr [kworker/2:0-cgroup_release]
     29 kworker/2:0H-ev [kworker/2:0H-events_highpri]
     30 cpuhp/4         [cpuhp/4]
    131 kauditd         [kauditd]
    133 oom_reaper      [oom_reaper]
    137 khugepaged      [khugepaged]
    138 kworker/R-kinte [kworker/R-kintegrityd]
   1375 dbus-daemon     /usr/bin/dbus-daemon --system --address=systemd: --nofork --nopidfile --systemd-activation --syslog-only
   1381 polkitd         /usr/lib/polkit-1/polkitd --no-debug --log-level=notice
   1383 smartd          /usr/sbin/smartd -n
   1449 NetworkManager  /usr/sbin/NetworkManager --no-daemon
   1590 cron            /usr/sbin/cron -f
   1643 ` + global.ProgBaseName + `        /usr/local/bin/` + global.ProgBaseName + ` receive --config ` + global.DefaultConfigRecv + `
   1948 kworker/9:3     [kworker/9:3]
   1983 krfcommd        [krfcommd]
   4206 ps              /usr/bin/ps -axo pid,comm,args
`,
			expectSyscall:       true,
			expectSyscallTgtPID: 1643,
		},
		{
			name:        "multiple receivers",
			configPath:  global.DefaultConfigRecv,
			programName: global.ProgBaseName,
			psOutput: `    PID COMMAND         COMMAND
      1 systemd         /sbin/init
     27 ksoftirqd/2     [ksoftirqd/2]
     28 kworker/2:0-cgr [kworker/2:0-cgroup_release]
     29 kworker/2:0H-ev [kworker/2:0H-events_highpri]
     30 cpuhp/4         [cpuhp/4]
    131 kauditd         [kauditd]
    133 oom_reaper      [oom_reaper]
    137 khugepaged      [khugepaged]
    138 kworker/R-kinte [kworker/R-kintegrityd]
   1375 dbus-daemon     /usr/bin/dbus-daemon --system --address=systemd: --nofork --nopidfile --systemd-activation --syslog-only
   1381 polkitd         /usr/lib/polkit-1/polkitd --no-debug --log-level=notice
   1383 smartd          /usr/sbin/smartd -n
   1449 NetworkManager  /usr/sbin/NetworkManager --no-daemon
   1590 cron            /usr/sbin/cron -f
   1643 ` + global.ProgBaseName + `        /usr/local/bin/` + global.ProgBaseName + ` receive --config ` + global.DefaultConfigRecv + `
   1743 ` + global.ProgBaseName + `        /usr/local/bin/` + global.ProgBaseName + ` receive --config ` + global.DefaultConfigRecv + `
   1948 kworker/9:3     [kworker/9:3]
   1983 krfcommd        [krfcommd]
   4206 ps              /usr/bin/ps -axo pid,comm,args
`,
			expectSyscall: false,
			expectedErr:   "found multiple running processes matching name",
		},
		{
			name:        "receiver and sender running",
			configPath:  global.DefaultConfigRecv,
			programName: global.ProgBaseName,
			psOutput: `    PID COMMAND         COMMAND
      1 systemd         /sbin/init
    138 kworker/R-kinte [kworker/R-kintegrityd]
   1375 dbus-daemon     /usr/bin/dbus-daemon --system --address=systemd: --nofork --nopidfile --systemd-activation --syslog-only
   1381 polkitd         /usr/lib/polkit-1/polkitd --no-debug --log-level=notice
   1383 smartd          /usr/sbin/smartd -n
   1449 NetworkManager  /usr/sbin/NetworkManager --no-daemon
   1590 cron            /usr/sbin/cron -f
   1592 ` + global.ProgBaseName + `        /usr/local/bin/` + global.ProgBaseName + ` receive --config ` + global.DefaultConfigRecv + `
   1613 ` + global.ProgBaseName + `        /usr/local/bin/` + global.ProgBaseName + ` send --config ` + global.DefaultConfigSend + `
   1948 kworker/9:3     [kworker/9:3]
   4206 ps              /usr/bin/ps -axo pid,comm,args
`,
			expectSyscall:       true,
			expectSyscallTgtPID: 1592,
		},
		{
			name:        "no running processes",
			configPath:  global.DefaultConfigRecv,
			programName: global.ProgBaseName,
			psOutput: `    PID COMMAND         COMMAND
      1 systemd         /sbin/init
    138 kworker/R-kinte [kworker/R-kintegrityd]
   1375 dbus-daemon     /usr/bin/dbus-daemon --system --address=systemd: --nofork --nopidfile --systemd-activation --syslog-only
   1381 polkitd         /usr/lib/polkit-1/polkitd --no-debug --log-level=notice
   1383 smartd          /usr/sbin/smartd -n
   1449 NetworkManager  /usr/sbin/NetworkManager --no-daemon
   1590 cron            /usr/sbin/cron -f
   1948 kworker/9:3     [kworker/9:3]
   4206 ps              /usr/bin/ps -axo pid,comm,args
`,
			expectSyscall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCmdExec := cmdCombinedOutput
			cmdCombinedOutput = func(cmd *exec.Cmd) (out []byte, err error) {
				return []byte(tt.psOutput), nil
			}
			defer func() { cmdCombinedOutput = origCmdExec }()

			var syscallCalled bool
			var targetedPID int
			origKill := syscallKill
			syscallKill = func(pid int, sys syscall.Signal) error {
				targetedPID = pid
				syscallCalled = true
				return nil
			}
			defer func() { syscallKill = origKill }()

			err := IssueLiveSigningKeyReload(tt.configPath, tt.programName)
			if err != nil && tt.expectedErr == "" {
				t.Fatalf("expected no error from reload issuance, but got %v", err)
			}
			if err == nil && tt.expectedErr != "" {
				t.Fatalf("expected error '%s', but got nil", tt.expectedErr)
			}
			if err != nil && tt.expectedErr != "" {
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf("expected error '%s' but got '%v'", tt.expectedErr, err)
				} else {
					return
				}
			}

			if syscallCalled != tt.expectSyscall {
				t.Fatalf("expected syscall %v - got %v", tt.expectSyscall, syscallCalled)
			}
			if targetedPID != tt.expectSyscallTgtPID {
				t.Errorf("expected syscall to target pid %d - but got pid %d", tt.expectSyscallTgtPID, targetedPID)
			}
		})
	}
}
