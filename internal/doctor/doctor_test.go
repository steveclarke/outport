package doctor

import "testing"

func TestRunnerAllPass(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "check1",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "check1", Status: Pass, Message: "ok"}
		},
	})
	r.Add(Check{
		Name:     "check2",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "check2", Status: Pass, Message: "ok"}
		},
	})
	results := r.Run()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if HasFailures(results) {
		t.Error("expected no failures")
	}
}

func TestRunnerWithWarn(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "warn-check",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "warn-check", Status: Warn, Message: "warning", Fix: "do something"}
		},
	})
	results := r.Run()
	if results[0].Status != Warn {
		t.Errorf("expected Warn, got %v", results[0].Status)
	}
	if HasFailures(results) {
		t.Error("warnings should not count as failures")
	}
}

func TestRunnerWithFail(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "fail-check",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "fail-check", Status: Fail, Message: "broken", Fix: "fix it"}
		},
	})
	results := r.Run()
	if results[0].Status != Fail {
		t.Errorf("expected Fail, got %v", results[0].Status)
	}
	if !HasFailures(results) {
		t.Error("expected failures")
	}
}

func TestRunnerMixed(t *testing.T) {
	r := &Runner{}
	r.Add(Check{Name: "a", Category: "Cat1", Run: func() *Result {
		return &Result{Name: "a", Status: Pass, Message: "ok"}
	}})
	r.Add(Check{Name: "b", Category: "Cat1", Run: func() *Result {
		return &Result{Name: "b", Status: Warn, Message: "meh", Fix: "try this"}
	}})
	r.Add(Check{Name: "c", Category: "Cat2", Run: func() *Result {
		return &Result{Name: "c", Status: Fail, Message: "bad", Fix: "fix it"}
	}})
	results := r.Run()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "a" || results[1].Name != "b" || results[2].Name != "c" {
		t.Error("results should preserve insertion order")
	}
	if !HasFailures(results) {
		t.Error("expected failures due to fail check")
	}
}

func TestRunnerEmpty(t *testing.T) {
	r := &Runner{}
	results := r.Run()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if HasFailures(results) {
		t.Error("empty results should not have failures")
	}
}
