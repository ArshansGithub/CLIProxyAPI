package thinking

import (
	"fmt"
	"strings"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// refuseUnsupportedEffort is the no-substitution.refuse-unsupported-effort
// switch (fork-only; docs/fork/specs/2026-10-07-no-substitution-design.md).
// It is set from the config at startup and on every reload.
var refuseUnsupportedEffort atomic.Bool

// SetRefuseUnsupportedEffort turns the refusal on or off for the process.
func SetRefuseUnsupportedEffort(on bool) { refuseUnsupportedEffort.Store(on) }

// RefuseUnsupportedEffort reports whether an explicit effort the model does
// not offer is refused with a 400 instead of being clamped, remapped or dropped.
func RefuseUnsupportedEffort() bool { return refuseUnsupportedEffort.Load() }

// refuseEffort builds the 400 for a refused effort and logs it at Error so
// the refusal is never only a warning an operator has to go looking for.
func refuseEffort(provider, model, requested string, supported []string) *ThinkingError {
	offered := strings.Join(supported, ", ")
	log.WithFields(log.Fields{
		"provider":  provider,
		"model":     model,
		"requested": requested,
		"supported": offered,
	}).Error("no-substitution: effort refused |")
	msg := fmt.Sprintf("effort_unsupported: model %s does not offer effort %q; it offers %s", model, requested, offered)
	return NewThinkingErrorWithModel(ErrLevelNotSupported, msg, model)
}
