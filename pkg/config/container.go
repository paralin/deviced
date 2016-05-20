package config

import (
	"math"
	"strings"
)

type TargetContainer struct {
	// unique ID
	Id string
	// [namespace/]name no version
	Image string
	// acceptable version tags, in order of priority
	Versions []string
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
}
