// Package version reports the version embedded by the Go toolchain.
package version

import "runtime/debug"

var readBuildInfo = debug.ReadBuildInfo

// Current returns the module version, or dev for an unversioned local build.
func Current() string {
	info, ok := readBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}
