package cmd

import (
	"errors"
	"testing"
)

func TestErrorHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no config", errors.New("No outport.yml found in /foo"), "Run: outport init"},
		{"not registered", errors.New(`project "myapp" (instance "main") is not registered`), "Run: outport up"},
		{"no ports", errors.New("No ports allocated for this project"), "Run: outport up"},
		{"unrelated error", errors.New("something went wrong"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorHint(tt.err)
			if got != tt.want {
				t.Errorf("ErrorHint(%q) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
