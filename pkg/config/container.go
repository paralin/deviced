package config

import (
	"math"
	"strings"

	dc "github.com/fsouza/go-dockerclient"
)

type TargetContainer struct {
	// unique ID
	Id string `yaml:"id"`
	// [namespace/]name no version
	Image string `yaml:"image"`
	// acceptable version tags, in order of priority
	Versions               []string            `yaml:"versions"`
	UseAnyVersion          bool                `yaml:"useAnyVersion,omitempty"`
	RestartExited          bool                `yaml:"restartExited"`
	DockerConfig           dc.Config           `yaml:"dockerConfig,omitempty"`
	DockerHostConfig       dc.HostConfig       `yaml:"dockerHostConfig,omitempty"`
	DockerNetworkingConfig dc.NetworkingConfig `yaml:"dockerNetworkingConfig,omitempty"`
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
	AllowSelfDelete bool `yaml:"allowSelfDelete"`
}
