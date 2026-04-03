package portinfo

import (
	"regexp"
	"strconv"
	"strings"
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
