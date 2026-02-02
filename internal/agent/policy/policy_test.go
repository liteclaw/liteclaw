package policy

import (
	"testing"
)

func TestPolicyMatcher(t *testing.T) {
	tests := []struct {
		name    string
		policy  ToolPolicy
		tool    string
		allowed bool
	}{
		{
			name:    "Allow all by default",
			policy:  ToolPolicy{},
			tool:    "exec",
			allowed: true,
		},
		{
			name: "Deny specific tool",
			policy: ToolPolicy{
				Deny: []string{"exec"},
			},
			tool:    "exec",
			allowed: false,
		},
		{
			name: "Allow specific tool",
			policy: ToolPolicy{
				Allow: []string{"read"},
			},
			tool:    "read",
			allowed: true,
		},
		{
			name: "Deny overrides allow",
			policy: ToolPolicy{
				Allow: []string{"exec"},
				Deny:  []string{"exec"},
			},
			tool:    "exec",
			allowed: false,
		},
		{
			name: "Wildcard allow",
			policy: ToolPolicy{
				Allow: []string{"file_*"},
			},
			tool:    "file_read",
			allowed: true,
		},
		{
			name: "Wildcard deny",
			policy: ToolPolicy{
				Deny: []string{"admin_*"},
			},
			tool:    "admin_tool",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := tt.policy.Compile()
			if got := matcher(tt.tool); got != tt.allowed {
				t.Errorf("Matcher(%q) = %v, want %v", tt.tool, got, tt.allowed)
			}
		})
	}
}
