package rekvizitupaieska

import "runtime/debug"

const (
	modulePath   = "github.com/pauliusgin/rekvizitupaieska"
	develVersion = "devel"

	// goDevelVersion is what the Go toolchain records for a module built from a source
	// checkout rather than resolved at a tagged version. Its parentheses are not legal inside a
	// User-Agent product token, so it is reported as develVersion instead.
	goDevelVersion = "(devel)"
)

// userAgent names this library at the version the caller's binary was built against, so the
// version cannot drift from the git tag the way a hand-maintained constant would.
var userAgent = buildUserAgent(readLibraryVersion())

func buildUserAgent(version string) string {
	return "rekvizitupaieska/" + version + " (+https://" + modulePath + ")"
}

func readLibraryVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return develVersion
	}
	return libraryVersionFromBuildInfo(info)
}

// libraryVersionFromBuildInfo looks in two places because the library is the main module only
// when its own tests run; in a caller's binary it is one of the dependencies.
func libraryVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return develVersion
	}
	if info.Main.Path == modulePath {
		return reportableVersion(info.Main.Version)
	}
	for _, dependency := range info.Deps {
		if dependency.Path != modulePath {
			continue
		}
		if dependency.Replace != nil {
			return reportableVersion(dependency.Replace.Version)
		}
		return reportableVersion(dependency.Version)
	}
	return develVersion
}

func reportableVersion(version string) string {
	if version == "" || version == goDevelVersion {
		return develVersion
	}
	return version
}
