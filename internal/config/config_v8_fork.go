package config

// forkRootSections are the privacy-locked fork's own top-level config sections.
// They have no v8 home, so the v8 layout migration must treat them as known
// roots rather than unrecognized legacy sections. claude-code.cache-keepalive
// needs no row: it rides the upstream claude-code prefix into
// oauth.providers.claude.claude-code.
var forkRootSections = []string{"egress", "usage-cache-stats", "overload-retry"}
