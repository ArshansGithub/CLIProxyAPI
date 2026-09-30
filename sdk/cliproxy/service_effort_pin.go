package cliproxy

import (
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/effortpin"
	log "github.com/sirupsen/logrus"
)

// applyEffortPinConfig installs the effort-pin block on the process store at
// startup and on every config reload. A reload resets the watch page's
// runtime switch to the configured value.
func (s *Service) applyEffortPinConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	models := cfg.EffortPin.NormalizedModels()
	effortpin.Default().Configure(cfg.EffortPin.Enabled, models, cfg.EffortPin.AllowBare)
	if cfg.EffortPin.Enabled {
		log.WithFields(log.Fields{"model": models, "mode": map[bool]string{true: "allow-bare", false: "refuse-bare"}[cfg.EffortPin.AllowBare]}).Info("effort-pin: bare ids for pinned models are checked |")
	}
}
