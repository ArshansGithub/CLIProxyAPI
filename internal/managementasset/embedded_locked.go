//go:build locked

package managementasset

import _ "embed"

//go:embed panel/management.html
var embeddedPanel []byte

//go:embed panel/PANEL_VERSION
var panelVersionText string

// EmbeddedPanel returns the reviewed panel bundle compiled into this binary.
func EmbeddedPanel() ([]byte, bool) { return embeddedPanel, len(embeddedPanel) > 0 }

// PanelVersion returns the PANEL_VERSION pin text.
func PanelVersion() string { return panelVersionText }
