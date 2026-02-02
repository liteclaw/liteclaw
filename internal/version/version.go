// Package version provides version information for LiteClaw.
package version

// These variables are set at build time via ldflags.
var (
	// Version is the semantic version of LiteClaw.
	Version = "0.1.0-dev"

	// Commit is the git commit hash.
	Commit = "unknown"

	// BuildDate is the build timestamp.
	BuildDate = "unknown"
)
