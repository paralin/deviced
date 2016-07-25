package arch

import (
	"fmt"
	"runtime"
)

/*
	Returns the architecture tag for this platform.
	This will just be "arm" for arm and blank otherwise.
	This is for the Docker image stuff.
*/
func GetArchTagSuffix() string {
	arch := runtime.GOARCH
	switch arch {
	case
		"arm":
		return "-arm"
	case "arm64":
		return "-arm"
	}
	return ""
}

func AppendArchTagSuffix(versions []string) []string {
	suffix := GetArchTagSuffix()
	if suffix == "" {
		return versions
	}
	res := make([]string, len(versions))
	for i, st := range versions {
		res[i] = fmt.Sprintf("%s%s", st, suffix)
	}
	return res
}
