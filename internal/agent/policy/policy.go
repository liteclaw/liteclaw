package policy

import (
	"regexp"
	"strings"
)

// ToolPolicy defines which tools are allowed or denied.
type ToolPolicy struct {
	Allow []string `json:"allow" yaml:"allow" mapstructure:"allow"`
	Deny  []string `json:"deny" yaml:"deny" mapstructure:"deny"`
}

// Matcher is a function that checks if a tool name is allowed.
type Matcher func(name string) bool

// Compile compiles the policy into a Matcher function.
func (p *ToolPolicy) Compile() Matcher {
	denyPatterns := compilePatterns(p.Deny)
	allowPatterns := compilePatterns(p.Allow)

	return func(name string) bool {
		name = strings.ToLower(strings.TrimSpace(name))

		// 1. Check deny list first
		if matchesAny(name, denyPatterns) {
			return false
		}

		// 2. If allow list is empty, allow all by default (unless denied)
		if len(allowPatterns) == 0 {
			return true
		}

		// 3. Otherwise, check allow list
		return matchesAny(name, allowPatterns)
	}
}

type pattern struct {
	all   bool
	exact string
	regex *regexp.Regexp
}

func compilePatterns(raw []string) []pattern {
	var compiled []pattern
	for _, r := range raw {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" {
			continue
		}
		if r == "*" {
			compiled = append(compiled, pattern{all: true})
			continue
		}
		if !strings.Contains(r, "*") {
			compiled = append(compiled, pattern{exact: r})
			continue
		}

		// Regex for wildcard
		expr := "^" + strings.ReplaceAll(regexp.QuoteMeta(r), "\\*", ".*") + "$"
		if re, err := regexp.Compile(expr); err == nil {
			compiled = append(compiled, pattern{regex: re})
		}
	}
	return compiled
}

func matchesAny(name string, patterns []pattern) bool {
	for _, p := range patterns {
		if p.all {
			return true
		}
		if p.exact != "" && name == p.exact {
			return true
		}
		if p.regex != nil && p.regex.MatchString(name) {
			return true
		}
	}
	return false
}
