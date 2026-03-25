// Package doctor implements the diagnostic system behind the "outport doctor"
// command. It defines a set of health checks that verify every layer of the
// Outport stack: DNS resolver configuration, daemon process status, TLS
// certificate validity, registry integrity, and project-level configuration.
//
// Checks are organized as Check values with a Name, Category, and Run function.
// A Runner executes checks sequentially and collects Results, each with a
// pass/warn/fail status and an optional fix suggestion. The system-level checks
// (SystemChecks) verify the shared infrastructure, while project-level checks
// (ProjectChecks) validate a specific project's config and port allocations.
package doctor

// Status represents the outcome of a health check.
type Status int

const (
	Pass Status = iota
	Warn
	Fail
)

// String returns the lowercase status label for JSON output.
func (s Status) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	default:
		return "unknown"
	}
}

// Check is a single diagnostic check.
type Check struct {
	Name     string
	Category string
	Run      func() *Result
}

// Result is the outcome of running a Check.
type Result struct {
	Name     string
	Category string
	Status   Status
	Message  string
	Fix      string
}

// Runner collects and executes checks sequentially.
type Runner struct {
	checks []Check
}

// Add appends a check to the runner.
func (r *Runner) Add(c Check) {
	r.checks = append(r.checks, c)
}

// Run executes all checks in order and returns the results.
func (r *Runner) Run() []Result {
	results := make([]Result, 0, len(r.checks))
	for _, c := range r.checks {
		res := c.Run()
		res.Category = c.Category
		results = append(results, *res)
	}
	return results
}

// HasFailures returns true if any result has Fail status.
func HasFailures(results []Result) bool {
	for _, r := range results {
		if r.Status == Fail {
			return true
		}
	}
	return false
}
