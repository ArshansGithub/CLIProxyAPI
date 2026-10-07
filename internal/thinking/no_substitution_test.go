package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
)

func TestRefuseUnsupportedEffortSwitch(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	if thinking.RefuseUnsupportedEffort() {
		t.Fatal("default must be off")
	}
	thinking.SetRefuseUnsupportedEffort(true)
	if !thinking.RefuseUnsupportedEffort() {
		t.Fatal("switch did not turn on")
	}
}
