// Package version holds the vAssessment build version, injected at build
// time via -ldflags (see vassessment/Makefile).
package version

// Version is set via -ldflags at build time. It stays "dev" for local
// `go build`/`go run` invocations that skip the Makefile.
var Version = "dev"
