package commands

import (
	"bytes"
	"testing"

	"github.com/liteclaw/liteclaw/internal/version"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	version.Version = "1.2.3"
	version.Commit = "abcdef"
	version.BuildDate = "2023-10-27"

	cmd := NewVersionCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)

	out := b.String()
	assert.Contains(t, out, "LiteClaw 1.2.3")
	assert.Contains(t, out, "Commit: abcdef")
	assert.Contains(t, out, "Built:  2023-10-27")
}
