package state

import (
	dct "github.com/docker/docker/api/types"
)

type RunningContainer struct {
	// The corresponding deviced ID
	DevicedID    string         `yaml:"devicedId"`
	Image        string         `yaml:"image"`
	ImageTag     string         `yaml:"imageTag"`
	Score        uint           `yaml:"score,omitempty"`
	ApiContainer *dct.Container `yaml:"apiContainer"`
}
