package portinfo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// lsofEntry is a raw port→PID mapping extracted from lsof output.
type lsofEntry struct {
	PID         int
	ProcessName string
	Port        int
}

// portPattern matches the port number at the end of lsof NAME fields like:
//
//	127.0.0.1:13542, *:3000, [::1]:8080
var portPattern = regexp.MustCompile(`:(\d+)\s*\(LISTEN\)`)

// parseLsofListening parses output from "lsof -iTCP -sTCP:LISTEN -P -n".
// Returns one entry per listening port/PID combination. Malformed lines are skipped.
func parseLsofListening(output string) ([]lsofEntry, error) {
	var entries []lsofEntry

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		// Look for the port pattern in the tail fields (NAME + state)
		tail := strings.Join(fields[8:], " ")
		match := portPattern.FindStringSubmatch(tail)
		if match == nil {
			continue
		}

		port, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		entries = append(entries, lsofEntry{
			PID:         pid,
			ProcessName: fields[0],
			Port:        port,
		})
	}

	return entries, nil
}

// psEntry holds parsed fields from a single ps output line.
type psEntry struct {
	PID     int
	PPID    int
	State   string
	RSS     int64 // kilobytes from ps
	Elapsed time.Duration
	Command string
}

// parsePsOutput parses output from "ps -p ... -o pid=,ppid=,stat=,rss=,etime=,command=".
// The etime field is a single token in [[dd-]hh:]mm:ss format.
// Returns a map of PID → psEntry. Malformed lines are skipped.
func parsePsOutput(output string) map[int]psEntry {
	entries := make(map[int]psEntry)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		// Minimum: pid, ppid, stat, rss, etime, command (1+ tokens) = 6
		if len(fields) < 6 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		state := fields[2]
		rss, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			continue
		}

		elapsed, err := parseEtime(fields[4])
		if err != nil {
			continue
		}

		command := strings.Join(fields[5:], " ")

		entries[pid] = psEntry{
			PID:     pid,
			PPID:    ppid,
			State:   state,
			RSS:     rss,
			Elapsed: elapsed,
			Command: command,
		}
	}

	return entries
}

// parseEtime parses the ps etime format: [[dd-]hh:]mm:ss.
// Examples: "00:00", "14:30", "02:14:30", "02-14:48:45"
func parseEtime(s string) (time.Duration, error) {
	var days, hours, minutes, seconds int

	// Split off days if present: "02-14:48:45" → days=2, rest="14:48:45"
	rest := s
	if i := strings.Index(s, "-"); i >= 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, fmt.Errorf("bad etime days: %q", s)
		}
		days = d
		rest = s[i+1:]
	}

	parts := strings.Split(rest, ":")
	switch len(parts) {
	case 2: // mm:ss
		m, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("bad etime minutes: %q", s)
		}
		sec, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("bad etime seconds: %q", s)
		}
		minutes, seconds = m, sec
	case 3: // hh:mm:ss
		h, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("bad etime hours: %q", s)
		}
		m, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("bad etime minutes: %q", s)
		}
		sec, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("bad etime seconds: %q", s)
		}
		hours, minutes, seconds = h, m, sec
	default:
		return 0, fmt.Errorf("bad etime format: %q", s)
	}

	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second, nil
}

// parseLsofCwd parses output from "lsof -a -d cwd -p ...".
// Returns a map of PID → working directory path.
func parseLsofCwd(output string) map[int]string {
	cwds := make(map[int]string)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		// NAME field starts at index 8 — join to handle paths with spaces
		cwds[pid] = strings.Join(fields[8:], " ")
	}

	return cwds
}
