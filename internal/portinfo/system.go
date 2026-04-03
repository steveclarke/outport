// internal/portinfo/system.go
package portinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

// SystemScanner implements Scanner by shelling out to real system commands.
type SystemScanner struct{}

func (s SystemScanner) ListeningPorts() (string, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("lsof: %w", err)
	}
	return string(out), nil
}

func (s SystemScanner) ProcessInfo(pids []int) (string, error) {
	if len(pids) == 0 {
		return "", nil
	}
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = fmt.Sprintf("%d", pid)
	}
	out, err := exec.Command("ps", "-p", strings.Join(pidStrs, ","),
		"-o", "pid=,ppid=,stat=,rss=,lstart=,command=").Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	return string(out), nil
}

func (s SystemScanner) WorkingDirs(pids []int) (string, error) {
	if len(pids) == 0 {
		return "", nil
	}
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = fmt.Sprintf("%d", pid)
	}
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(pidStrs, ",")).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("lsof cwd: %w", err)
	}
	return string(out), nil
}
