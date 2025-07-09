package version

import (
	"runtime"
	"runtime/debug"
)

var (
	Version   string // Set via ldflags.
	Branch    string
	BuildUser string
	BuildDate string

	Revision  = getRevision()
	GoVersion = runtime.Version()
	GoOS      = runtime.GOOS
	GoArch    = runtime.GOARCH
)

func GetVersion() string {
	if Version != "" {
		return Version
	}

	return Revision
}

func getRevision() string {
	rev := "unknown"

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return rev
	}

	modified := false

	for _, v := range buildInfo.Settings {
		switch v.Key {
		case "vcs.revision":
			if len(v.Value) > 7 {
				rev = v.Value[:7]
			} else {
				rev = v.Value
			}

		case "vcs.modified":
			if v.Value == "true" {
				modified = true
			}
		}
	}

	if modified {
		return rev + "-dirty"
	}

	return rev
}
