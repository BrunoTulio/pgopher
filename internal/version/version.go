package version

import "runtime"

var (
	Version   = "dev"             //  via ldflags
	GitCommit = "none"            //  via ldflags
	BuildDate = "unknown"         //  via ldflags
	GoVersion = runtime.Version() // Runtime
)

type Info struct {
	Version   string
	GitCommit string
	BuildDate string
	GoVersion string
	OS        string
	Arch      string
}

func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
