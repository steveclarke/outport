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
	PID       int
	PPID      int
	State     string
	RSS       int64 // kilobytes from ps
	StartTime time.Time
	Command   string
}

// parsePsOutput parses output from "ps -p ... -o pid=,ppid=,stat=,rss=,lstart=,command=".
// The lstart field is 5 tokens wide (e.g., "Thu Mar 27 09:15:00 2026").
// Returns a map of PID → psEntry. Malformed lines are skipped.
func parsePsOutput(output string) map[int]psEntry {
	entries := make(map[int]psEntry)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		// Minimum: pid, ppid, stat, rss, lstart (5 tokens), command (1+ tokens) = 9
		if len(fields) < 9 {
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

		// lstart is 5 tokens but format varies by locale:
		//   US:  "Thu Mar 27 09:15:00 2026"  (Mon Jan _2 15:04:05 2006)
		//   EU:  "Tue 31 Mar 22:38:00 2026"  (Mon _2 Jan 15:04:05 2006)
		lstartStr := strings.Join(fields[4:9], " ")
		startTime, err := parseLstart(lstartStr)
		if err != nil {
			continue
		}

		command := strings.Join(fields[9:], " ")

		entries[pid] = psEntry{
			PID:       pid,
			PPID:      ppid,
			State:     state,
			RSS:       rss,
			StartTime: startTime,
			Command:   command,
		}
	}

	return entries
}

// lstartFormats lists the known ps lstart formats across locales.
var lstartFormats = []string{
	"Mon Jan _2 15:04:05 2006", // US: "Thu Mar 27 09:15:00 2026"
	"Mon _2 Jan 15:04:05 2006", // EU: "Tue 31 Mar 22:38:00 2026"
}

// parseLstart tries each known lstart format and returns the first that succeeds.
func parseLstart(s string) (time.Time, error) {
	for _, layout := range lstartFormats {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized lstart format: %q", s)
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
