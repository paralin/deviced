package config

import (
	"math"
	"strings"

	dcapi "github.com/docker/docker/api/types/container"
	dcnapi "github.com/docker/docker/api/types/network"
	"github.com/fuserobotics/deviced/pkg/arch"
)

type TargetContainer struct {
	// unique ID
	Id string `yaml:"id"`
	// [namespace/]name no version
	Image string `yaml:"image"`
	// acceptable version tags, in order of priority
	Versions               []string                `yaml:"versions"`
	UseAnyVersion          bool                    `yaml:"useAnyVersion,omitempty"`
	NoArchTag              bool                    `yaml:"noArchTag,omitempty"`
	RestartExited          bool                    `yaml:"restartExited"`
	DockerConfig           dcapi.Config            `yaml:"dockerConfig,omitempty"`
	DockerHostConfig       dcapi.HostConfig        `yaml:"dockerHostConfig,omitempty"`
	DockerNetworkingConfig dcnapi.NetworkingConfig `yaml:"dockerNetworkingConfig,omitempty"`
	LifecycleHooks         LifecycleHookSet        `yaml:"lifecycleHooks,omitempty"`
}

type LifecycleHookSet struct {
	OnStop []LifecycleHook
}

type LifecycleHook struct {
	Exec *LifecycleExecHook
}

type LifecycleExecHook struct {
	Command []string
	Timeout string
}

func (tc *TargetContainer) ContainerVersionScore(version string) uint {
	vers := arch.AppendArchTagSuffix(tc.Versions)
	for idx, ver := range vers {
		if strings.EqualFold(ver, version) {
			return uint(idx)
		}
	}
	return math.MaxUint16
}

type ContainerWorkerConfig struct {
	AllowSelfDelete bool `yaml:"allowSelfDelete"`
}
