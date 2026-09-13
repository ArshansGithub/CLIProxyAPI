//go:build !locked

// Package lockedbuild reports whether the binary was compiled with the locked tag.
package lockedbuild

// Enabled is true only in builds compiled with `-tags locked`.
const Enabled = false
