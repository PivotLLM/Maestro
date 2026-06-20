package maestro

import (
	"embed"
)

// EmbeddedReference contains all files from the docs/ai directory
//go:embed docs/ai/* docs/ai/phases/* docs/ai/templates/*
var EmbeddedReference embed.FS
