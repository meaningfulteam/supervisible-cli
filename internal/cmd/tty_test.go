package cmd

import "testing"

// These are smoke tests: TTY detection is platform-dependent and we don't have a
// portable way to fake a TTY in CI. Just assert the helpers don't panic and
// return bool. In the standard `go test` run, stdin/stdout are not TTYs.
func TestTTYHelpersDoNotPanic(t *testing.T) {
	if isStdinInteractive() {
		t.Log("stdin is interactive (unusual under `go test`, ok)")
	}
	if isStdoutInteractive() {
		t.Log("stdout is interactive (unusual under `go test`, ok)")
	}
}
