// Package console provides an embedded copy of the Hive web console
// (SvelteKit static build). The files are embedded at compile time
// from console/build/ via go:embed and served by the HTTP handler.
package console

import "embed"

//go:embed all:build
var Files embed.FS
