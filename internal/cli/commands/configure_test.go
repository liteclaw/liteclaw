package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigureCommand(t *testing.T) {
	cmd := NewConfigureCommand()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// configure is now a placeholder pointing to onboard
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, b.String(), "LiteClaw Configuration")
	assert.Contains(t, b.String(), "liteclaw onboard")
}
