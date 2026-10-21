package rules

import _ "embed"

// Boolean contains the egglog rules bundled with the command-line tool.
//
//go:embed bool.egg
var Boolean string
