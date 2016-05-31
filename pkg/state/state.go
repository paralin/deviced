package state

import (
	dc "github.com/fsouza/go-dockerclient"
)

type RunningContainer struct {
	// The corresponding deviced ID
	DevicedID    string            `yaml:"devicedId"`
	Image        string            `yaml:"image"`
	ImageTag     string            `yaml:"imageTag"`
	Score        uint              "score,omitempty"
	ApiContainer *dc.APIContainers `yaml:"apiContainer"`
}
