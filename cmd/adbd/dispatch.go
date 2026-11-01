// Process spawning lives here and nowhere else in the router loop.
//
// Why a separate file: the router must be provably unable to execute anything on
// its own (truth.md AC-003). route.go imports no subprocess package at all; the
// single seam where the router reaches the outside world is the Dispatcher
// interface, and the only production implementation is this one -- `paseo send`,
// treated as an opaque external CLI (no import, no fork of its code, argv only).
package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// PaseoDispatcher hands a message to a paseo worker agent.
type PaseoDispatcher struct {
	// Bin is the paseo executable; empty means "paseo" from PATH.
	Bin string
}

// compile-time proof that the production dispatcher satisfies the seam.
var _ Dispatcher = PaseoDispatcher{}

// Dispatch sends one guarded text to one agent. The text is passed as a single
// argv element, so no shell ever parses it.
func (d PaseoDispatcher) Dispatch(thread, agentID, text string) error {
	bin := strings.TrimSpace(d.Bin)
	if bin == "" {
		bin = "paseo"
	}
	out, err := exec.Command(bin, "send", agentID, text).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dispatcher %s send %s (thread %s): %w (output: %s)",
			bin, agentID, thread, err, strings.TrimSpace(string(out)))
	}
	return nil
}
