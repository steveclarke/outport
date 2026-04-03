package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveclarke/outport/internal/portinfo"
	"github.com/steveclarke/outport/internal/ui"
)

var (
	killOrphansFlag bool
	killForceFlag   bool
)

var portsKillCmd = &cobra.Command{
	Use:   "kill <service-or-port>",
	Short: "Kill the process on a port",
	Long:  "Kills the process listening on a port. Target can be a service name (requires project context) or a port number. Use --orphans to kill all orphaned dev processes.",
	Args: func(cmd *cobra.Command, args []string) error {
		if killOrphansFlag {
			if len(args) > 0 {
				return FlagErrorf("--orphans does not accept arguments")
			}
			return nil
		}
		if len(args) == 0 {
			return FlagErrorf("requires a service name or port number")
		}
		if len(args) > 1 {
			return FlagErrorf("too many arguments")
		}
		return nil
	},
	RunE: runPortsKill,
}

func init() {
	portsKillCmd.Flags().BoolVar(&killOrphansFlag, "orphans", false, "kill all orphaned dev processes")
	portsKillCmd.Flags().BoolVar(&killForceFlag, "force", false, "skip confirmation prompt")
	portsCmd.AddCommand(portsKillCmd)
}

// killCandidate holds the details of a process selected for killing.
type killCandidate struct {
	port int
	pid  int
	name string
	cmd  string
}

// killJSON is the JSON output for a single kill result.
type killJSON struct {
	Port    int              `json:"port"`
	Process *portProcessJSON `json:"process"`
	Killed  bool             `json:"killed"`
}

// killOrphansJSON is the JSON output for --orphans results.
type killOrphansJSON struct {
	Killed []killJSON `json:"killed"`
	Failed []killJSON `json:"failed"`
}

func runPortsKill(cmd *cobra.Command, args []string) error {
	if jsonFlag && !killForceFlag {
		return FlagErrorf("--json requires --force (no interactive prompt in JSON mode)")
	}

	if killOrphansFlag {
		return runKillOrphans(cmd)
	}

	port, err := resolveKillTarget(args[0])
	if err != nil {
		return err
	}

	procs, err := portinfo.ScanPorts([]int{port}, portScanner)
	if err != nil {
		return fmt.Errorf("scanning port %d: %w", port, err)
	}

	if len(procs) == 0 {
		if jsonFlag {
			return writeJSON(cmd, killJSON{Port: port, Killed: false}, "no process found")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No process listening on port %d.\n", port)
		return nil
	}

	// Pick the lowest PID (likely the parent process)
	target := procs[0]
	for _, p := range procs[1:] {
		if p.PID < target.PID {
			target = p
		}
	}

	candidate := killCandidate{
		port: port,
		pid:  target.PID,
		name: target.Name,
		cmd:  target.Command,
	}

	if len(procs) > 1 {
		fmt.Fprintf(cmd.OutOrStdout(), "%d processes on port %d — targeting lowest PID:\n", len(procs), port)
		for _, p := range procs {
			marker := "  "
			if p.PID == target.PID {
				marker = "→ "
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %sPID %d  %s\n", marker, p.PID, p.Name)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	return killWithConfirmation(cmd, candidate, target)
}

func runKillOrphans(cmd *cobra.Command) error {
	procs, err := portinfo.Scan(portScanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}

	// Filter to orphans and zombies only
	var orphans []portinfo.ProcessInfo
	for _, p := range procs {
		if p.IsOrphan || p.IsZombie {
			orphans = append(orphans, p)
		}
	}

	if len(orphans) == 0 {
		if jsonFlag {
			return writeJSON(cmd, killOrphansJSON{}, "no orphaned processes found")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No orphaned dev processes found.")
		return nil
	}

	w := cmd.OutOrStdout()

	if !killForceFlag {
		fmt.Fprintf(w, "Found %d orphaned %s:\n\n",
			len(orphans), pluralize(len(orphans), "process", "processes"))
		for _, p := range orphans {
			printKillCandidate(w, killCandidate{
				port: p.Port,
				pid:  p.PID,
				name: p.Name,
				cmd:  p.Command,
			})
		}
		fmt.Fprintf(w, "\nKill all? [y/N] ")
		if !confirmYes(os.Stdin) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		fmt.Fprintln(w)
	}

	var killed, failed []killJSON
	for _, p := range orphans {
		pJSON := toPortProcessJSON(p)
		entry := killJSON{
			Port:    p.Port,
			Process: pJSON,
		}

		if err := portinfo.Kill(p.PID); err != nil {
			entry.Killed = false
			failed = append(failed, entry)
			if !jsonFlag {
				fmt.Fprintf(w, "  %s PID %d on port %d: %v\n",
					ui.WarnStyle.Render("✗"), p.PID, p.Port, err)
			}
			continue
		}

		// Brief wait then check if it survived
		time.Sleep(200 * time.Millisecond)
		if processAlive(p.PID) {
			entry.Killed = false
			failed = append(failed, entry)
			if !jsonFlag {
				fmt.Fprintf(w, "  %s PID %d on port %d survived SIGTERM — try: sudo kill -9 %d\n",
					ui.WarnStyle.Render("✗"), p.PID, p.Port, p.PID)
			}
		} else {
			entry.Killed = true
			killed = append(killed, entry)
			if !jsonFlag {
				fmt.Fprintf(w, "  %s Killed PID %d on port %d (%s)\n",
					ui.SuccessStyle.Render("✓"), p.PID, p.Port, p.Name)
			}
		}
	}

	if jsonFlag {
		result := killOrphansJSON{Killed: killed, Failed: failed}
		summary := fmt.Sprintf("%d killed, %d failed", len(killed), len(failed))
		return writeJSON(cmd, result, summary)
	}

	return nil
}

func killWithConfirmation(cmd *cobra.Command, candidate killCandidate, proc portinfo.ProcessInfo) error {
	w := cmd.OutOrStdout()

	if !killForceFlag {
		printKillCandidate(w, candidate)
		fmt.Fprintf(w, "\nKill this process? [y/N] ")
		if !confirmYes(os.Stdin) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		fmt.Fprintln(w)
	}

	if err := portinfo.Kill(candidate.pid); err != nil {
		if jsonFlag {
			return writeJSON(cmd, killJSON{
				Port:    candidate.port,
				Process: toPortProcessJSON(proc),
				Killed:  false,
			}, "kill failed: "+err.Error())
		}
		return fmt.Errorf("kill PID %d: %w", candidate.pid, err)
	}

	// Brief wait then check if process survived
	time.Sleep(200 * time.Millisecond)
	alive := processAlive(candidate.pid)

	if jsonFlag {
		return writeJSON(cmd, killJSON{
			Port:    candidate.port,
			Process: toPortProcessJSON(proc),
			Killed:  !alive,
		}, formatKillSummary(candidate, alive))
	}

	if alive {
		fmt.Fprintf(w, "%s PID %d survived SIGTERM — try: sudo kill -9 %d\n",
			ui.WarnStyle.Render("⚠"), candidate.pid, candidate.pid)
	} else {
		fmt.Fprintf(w, "%s Killed PID %d on port %d (%s)\n",
			ui.SuccessStyle.Render("✓"), candidate.pid, candidate.port, candidate.name)
	}

	return nil
}

// resolveKillTarget resolves a target argument to a port number.
// It tries parsing as a port number first, then as a service name via project context.
func resolveKillTarget(target string) (int, error) {
	// Try as a port number first
	if port, err := strconv.Atoi(target); err == nil {
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("port %d out of range (1-65535)", port)
		}
		return port, nil
	}

	// Try as a service name via project context
	ctx, err := loadProjectContext()
	if err != nil {
		return 0, fmt.Errorf("not a port number and no project context: %w", err)
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return 0, fmt.Errorf("no ports allocated — run 'outport up' first")
	}

	port, ok := alloc.Ports[target]
	if !ok {
		return 0, fmt.Errorf("service %q not found in project %q", target, ctx.Cfg.Name)
	}

	return port, nil
}

// printKillCandidate prints a one-line summary of the process to be killed.
func printKillCandidate(w io.Writer, c killCandidate) {
	line := fmt.Sprintf("  PID %d on port %d", c.pid, c.port)
	if c.name != "" {
		line += fmt.Sprintf("  (%s)", c.name)
	}
	if c.cmd != "" && c.cmd != c.name {
		line += fmt.Sprintf("  %s", truncate(c.cmd, 50))
	}
	fmt.Fprintln(w, line)
}

// confirmYes reads a line from r and returns true for y/yes (case-insensitive).
func confirmYes(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// processAlive checks if a process is still running by sending signal 0.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func formatKillSummary(c killCandidate, alive bool) string {
	if alive {
		return fmt.Sprintf("PID %d survived SIGTERM", c.pid)
	}
	return fmt.Sprintf("killed PID %d on port %d", c.pid, c.port)
}
