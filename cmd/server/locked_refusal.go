package main

import "github.com/router-for-me/CLIProxyAPI/v7/internal/lockedbuild"

// lockedBuildRefusal names the first startup option the locked build will not
// run with, or "" when there is none. Home mode dials Redis and its Home
// server outside the egress gate and accepts config (including the egress
// block) from that server; the Postgres, object and git token stores talk to
// the network through client libraries that never pass the gate. None of
// them is used by this fork's deployment, so the locked build refuses them
// rather than pretending the gate covers them.
func lockedBuildRefusal(homeMode, postgresStore, objectStore, gitStore bool) string {
	if !lockedbuild.Enabled {
		return ""
	}
	switch {
	case homeMode:
		return "locked build: home mode is disabled (it connects outside the egress gate and can rewrite egress config)"
	case postgresStore:
		return "locked build: the postgres token store is disabled (its client dials outside the egress gate)"
	case objectStore:
		return "locked build: the object token store is disabled (its client dials outside the egress gate)"
	case gitStore:
		return "locked build: the git token store is disabled (its client dials outside the egress gate)"
	}
	return ""
}
