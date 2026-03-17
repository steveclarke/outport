package config

import "testing"

func TestExpandVars(t *testing.T) {
	vars := map[string]string{
		"name":     "myapp",
		"instance": "xbjf",
		"rails.port": "3000",
		"rails.url":  "https://myapp.test",
	}

	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		// Simple substitution
		{
			name:     "simple var",
			template: "${name}",
			vars:     vars,
			want:     "myapp",
		},
		{
			name:     "var with surrounding text",
			template: "app-${name}-dev",
			vars:     vars,
			want:     "app-myapp-dev",
		},
		{
			name:     "dotted var",
			template: "port is ${rails.port}",
			vars:     vars,
			want:     "port is 3000",
		},
		{
			name:     "multiple vars",
			template: "${name}-${instance}",
			vars:     vars,
			want:     "myapp-xbjf",
		},

		// Default operator :-
		{
			name:     "default when empty",
			template: "${missing:-fallback}",
			vars:     vars,
			want:     "fallback",
		},
		{
			name:     "default when set",
			template: "${name:-fallback}",
			vars:     vars,
			want:     "myapp",
		},
		{
			name:     "default when var is empty string",
			template: "${empty:-fallback}",
			vars:     map[string]string{"empty": ""},
			want:     "fallback",
		},

		// Conditional operator :+
		{
			name:     "conditional when set",
			template: "${instance:+-${instance}}",
			vars:     vars,
			want:     "-xbjf",
		},
		{
			name:     "conditional when empty",
			template: "${instance:+-${instance}}",
			vars:     map[string]string{"instance": ""},
			want:     "",
		},
		{
			name:     "conditional when missing",
			template: "${missing:+present}",
			vars:     vars,
			want:     "",
		},
		{
			name:     "conditional with plain text",
			template: "${instance:+has instance}",
			vars:     vars,
			want:     "has instance",
		},

		// Real-world use cases
		{
			name:     "compose project name main instance",
			template: "outport-app${instance:+-${instance}}",
			vars:     map[string]string{"instance": ""},
			want:     "outport-app",
		},
		{
			name:     "compose project name worktree",
			template: "outport-app${instance:+-${instance}}",
			vars:     map[string]string{"instance": "xbjf"},
			want:     "outport-app-xbjf",
		},
		{
			name:     "nested var in conditional",
			template: "${instance:+${name}-${instance}}",
			vars:     map[string]string{"instance": "xbjf", "name": "myapp"},
			want:     "myapp-xbjf",
		},
		{
			name:     "nested var in default",
			template: "${missing:-${name}}",
			vars:     map[string]string{"name": "myapp"},
			want:     "myapp",
		},

		// Colon modifiers (e.g., :direct) — not operators
		{
			name:     "colon modifier treated as var name",
			template: "${rails.url:direct}/api",
			vars:     map[string]string{"rails.url:direct": "http://localhost:3000"},
			want:     "http://localhost:3000/api",
		},
		{
			name:     "colon modifier with simple var",
			template: "${rails.url:direct} and ${rails.url}",
			vars:     map[string]string{"rails.url:direct": "http://localhost:3000", "rails.url": "https://myapp.test"},
			want:     "http://localhost:3000 and https://myapp.test",
		},

		// Edge cases
		{
			name:     "no vars in template",
			template: "plain text",
			vars:     vars,
			want:     "plain text",
		},
		{
			name:     "empty template",
			template: "",
			vars:     vars,
			want:     "",
		},
		{
			name:     "literal dollar brace not a var",
			template: "cost is $5",
			vars:     vars,
			want:     "cost is $5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandVars(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("ExpandVars(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}
