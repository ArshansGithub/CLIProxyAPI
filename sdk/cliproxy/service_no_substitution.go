package cliproxy

import (
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	log "github.com/sirupsen/logrus"
)

// applyNoSubstitutionConfig installs the no-substitution switches at startup
// and on every config reload, beside applyEffortPinConfig. A reload with the
// block absent turns both off, which is the rollback path.
func (s *Service) applyNoSubstitutionConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	executor.SetStripFallbacks(cfg.NoSubstitution.StripFallbacks)
	thinking.SetRefuseUnsupportedEffort(cfg.NoSubstitution.RefuseUnsupportedEffort)
	log.WithFields(log.Fields{
		"strip_fallbacks":           cfg.NoSubstitution.StripFallbacks,
		"refuse_unsupported_effort": cfg.NoSubstitution.RefuseUnsupportedEffort,
	}).Info("no-substitution: switches applied |")
}
