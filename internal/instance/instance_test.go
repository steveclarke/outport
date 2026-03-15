package instance

import "testing"

func TestGenerateCode(t *testing.T) {
	used := map[string]bool{}
	code := GenerateCode(used)
	if len(code) != 4 {
		t.Fatalf("code length: got %d, want 4", len(code))
	}
	for _, c := range code {
		if !isConsonant(byte(c)) {
			t.Errorf("code %q contains non-consonant %c", code, c)
		}
	}
}

func TestGenerateCodeAvoidsCollisions(t *testing.T) {
	used := map[string]bool{}
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := GenerateCode(used)
		if codes[code] {
			t.Fatalf("duplicate code %q on iteration %d", code, i)
		}
		codes[code] = true
		used[code] = true
	}
}

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"main", false},
		{"feature-xyz", false},
		{"bkrm", false},
		{"agent-1", false},
		{"", true},
		{"UPPER", true},
		{"has space", true},
		{"has_underscore", true},
	}
	for _, tt := range tests {
		err := ValidateName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateName(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}
