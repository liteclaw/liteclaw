package commands

import (
	"bytes"
	"io"
	"os"
)

// CaptureStdout captures stdout during a function call
func CaptureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	fn()
	_ = w.Close()
	os.Stdout = old
	return <-outChan
}
