package portinfo

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// SystemLister implements Lister using gopsutil for native OS process inspection.
type SystemLister struct{}

// ListProcesses discovers all TCP ports in LISTEN state and returns process
// info for each. Uses gopsutil's net and process packages — no shelling out.
func (s SystemLister) ListProcesses() ([]ProcessInfo, error) {
	ctx := context.Background()

	conns, err := net.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return nil, err
	}

	// Collect unique PIDs for listening connections
	type portPID struct {
		port int
		pid  int
	}
	var listening []portPID
	seen := make(map[portPID]bool)

	for _, c := range conns {
		if c.Status != "LISTEN" || c.Pid == 0 {
			continue
		}
		pp := portPID{port: int(c.Laddr.Port), pid: int(c.Pid)}
		if seen[pp] {
			continue
		}
		seen[pp] = true
		listening = append(listening, pp)
	}

	// Look up process details for each unique PID
	pidCache := make(map[int]*processDetails)
	for _, pp := range listening {
		if _, ok := pidCache[pp.pid]; !ok {
			pidCache[pp.pid] = lookupProcess(ctx, pp.pid)
		}
	}

	// Build results
	var results []ProcessInfo
	for _, pp := range listening {
		details := pidCache[pp.pid]
		info := ProcessInfo{
			PID:     pp.pid,
			Port:    pp.port,
			PPID:    details.ppid,
			Name:    details.name,
			Command: details.cmdline,
			RSS:     details.rss,
			Elapsed: details.elapsed,
			State:   details.state,
			CWD:     details.cwd,
		}
		results = append(results, info)
	}

	return results, nil
}

// processDetails caches the result of querying a single process.
type processDetails struct {
	ppid    int
	name    string
	cmdline string
	rss     int64
	elapsed time.Duration
	state   string
	cwd     string
}

// lookupProcess gathers all available details for a PID. Individual field
// failures are non-fatal — we return whatever we can get.
func lookupProcess(ctx context.Context, pid int) *processDetails {
	d := &processDetails{}

	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return d
	}

	if ppid, err := p.PpidWithContext(ctx); err == nil {
		d.ppid = int(ppid)
	}
	if name, err := p.NameWithContext(ctx); err == nil {
		d.name = name
	}
	if cmdline, err := p.CmdlineWithContext(ctx); err == nil {
		d.cmdline = cmdline
	}
	if mem, err := p.MemoryInfoWithContext(ctx); err == nil && mem != nil {
		d.rss = int64(mem.RSS)
	}
	if createTime, err := p.CreateTimeWithContext(ctx); err == nil {
		d.elapsed = time.Since(time.UnixMilli(createTime))
	}
	if statuses, err := p.StatusWithContext(ctx); err == nil && len(statuses) > 0 {
		d.state = strings.Join(statuses, "")
	}
	if cwd, err := p.CwdWithContext(ctx); err == nil {
		d.cwd = cwd
	}

	return d
}
