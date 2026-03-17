package monitor

import "embed"

//go:embed templates/*
var TemplateFS embed.FS

//go:embed all:static
var StaticFS embed.FS
