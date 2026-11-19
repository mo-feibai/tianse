// Package version holds the one build-time constant the UI and the installer
// have to agree on.
//
// The number lives in the VERSION file at the repository root and is injected
// here by scripts/package.ps1 through -ldflags. It is a var rather than a const
// because the linker can only overwrite vars. A plain "go build" without the
// script keeps the placeholder, which is what a dev build should report.
package version

// Number is the release version, e.g. "0.2.0".
var Number = "dev"

// String returns the version for display. Dev builds say so explicitly instead
// of printing a number that no release ever had.
func String() string {
	if Number == "" || Number == "dev" {
		return "dev"
	}
	return Number
}
