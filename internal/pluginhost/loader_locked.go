//go:build locked

package pluginhost

import "errors"

// lockedLoader refuses to load any native plugin.
type lockedLoader struct{}

func defaultPluginLoader() pluginLoader { return lockedLoader{} }

func (lockedLoader) Open(file pluginFile, _ *Host) (pluginClient, error) {
	return nil, errors.New("pluginhost: native plugin loading is disabled in the locked build: " + file.Path)
}
