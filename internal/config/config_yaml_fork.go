package config

import (
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// isForkDefaultSection reports whether node is one of the fork's own sections
// holding exactly what the loader fills in when the section is absent. A save
// must not add a section the operator never set: written into a legacy file,
// the defaulted claude-code.cache-keepalive lands at its v8 path and the next
// v0 save then reads the whole file as v8.
func isForkDefaultSection(path []string, node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	switch strings.Join(path, ".") {
	case "claude-code.cache-keepalive":
		return yamlDecodesTo(path, node, ClaudeCodeCacheKeepaliveConfig{}.WithDefaults())
	case "usage-cache-stats":
		return yamlDecodesTo(path, node, UsageCacheStatsConfig{}.WithDefaults())
	case "egress":
		return yamlDecodesTo(path, node, EgressConfig{}.WithDefaults())
	case "overload-retry":
		return yamlDecodesTo(path, node, OverloadRetryConfig{})
	}
	return false
}

// yamlDecodesTo compares node with want after both pass through the pruning a
// new key gets before this check, so empty slices and zero fields that the
// merge drops compare as the loader would read them back.
func yamlDecodesTo[T any](path []string, node *yaml.Node, want T) bool {
	var wantNode yaml.Node
	if errEncode := wantNode.Encode(want); errEncode != nil {
		return false
	}
	pruneKnownDefaultsInNewNode(path, &wantNode)
	var got, wantPruned T
	if errDecode := node.Decode(&got); errDecode != nil {
		return false
	}
	if errDecode := wantNode.Decode(&wantPruned); errDecode != nil {
		return false
	}
	return reflect.DeepEqual(got, wantPruned)
}
