package config

import (
	"math"
	"strings"

	dc "github.com/fsouza/go-dockerclient"
)

type TargetContainer struct {
	// unique ID
	Id string `json:"id"`
	// [namespace/]name no version
	Image string `json:"image"`
	// acceptable version tags, in order of priority
	Versions               []string            `json:"versions"`
	UseAnyVersion          bool                `json:"useAnyVersion,omitempty"`
	RestartExited          bool                `json:"restartExited"`
	DockerConfig           dc.Config           `json:"dockerConfig,omitempty"`
	DockerHostConfig       dc.HostConfig       `json:"dockerHostConfig,omitempty"`
	DockerNetworkingConfig dc.NetworkingConfig `json:"dockerNetworkingConfig,omitempty"`
}

func (tc *TargetContainer) ContainerVersionScore(version string) uint {
	for idx, ver := range tc.Versions {
		if strings.EqualFold(ver, version) {
			return uint(idx)
		}
	}
	return math.MaxUint16
}

type ContainerWorkerConfig struct {
	AllowSelfDelete bool `json:"allowSelfDelete"`
}
