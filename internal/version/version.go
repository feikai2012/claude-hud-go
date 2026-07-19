// Package version exposes the build version, injected at build time via
// -ldflags "-X .../version.Version=<v>".
package version

// Version is the plugin version. Overridden at build time.
var Version = "0.0.0-dev"
