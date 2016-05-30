package config

import (
	"math"
	"strings"

	dc "github.com/fsouza/go-dockerclient"
)

type TargetContainer struct {
	// unique ID
	Id string "id"
	// [namespace/]name no version
	Image string "image"
	// acceptable version tags, in order of priority
	Versions []string "versions"
	// Use any version until the target version is available
	UseAnyVersion bool                      "useAnyVersion,omitempty"
	Options       dc.CreateContainerOptions "containerOptions,omitempty"
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
	// If true, manage everything automatically
	ManageAllContainers bool "manageAllContainers"
	AllowSelfDelete     bool "allowSelfDelete"
}
