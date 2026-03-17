// Package monitor provides embedded filesystem assets for templates and static files.
package monitor

import "embed"

// TemplatesFS holds the embedded templates/ directory.
//
//go:embed templates/*
var TemplatesFS embed.FS

// StaticFS holds the embedded static/ directory (including vendor/).
//
//go:embed static/*
var StaticFS embed.FS
