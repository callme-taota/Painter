package web

import _ "embed"

// ColorStyles is also used by the isolated identity consent page. Its styles are
// generated from the same registry as the public website and studio.
//
//go:embed frontend/src/color-tokens.css
var colorStyles string

func ColorStyles() string { return colorStyles }
