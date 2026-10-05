package config

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var embeddedVersion string

// Version is shared by local, Docker and release builds, and exposed by /api/version.
var Version = strings.TrimSpace(embeddedVersion)
