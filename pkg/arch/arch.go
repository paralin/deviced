package arch

import (
	"runtime"
)

/*
	Returns the architecture tag for this platform.
	This will just be "arm" for arm and blank otherwise.
	This is for the Docker image stuff.
*/
func GetArchExtraTag() string {
	arch := runtime.GOARCH
	switch arch {
	case
		"arm":
		return "arm"
	}
	return ""
}
