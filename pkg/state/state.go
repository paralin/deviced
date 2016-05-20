package state

import (
	"fmt"
	"io/ioutil"
	"os"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/go-yaml/yaml"
)

type RunningContainer struct {
	// The corresponding deviced ID
	DevicedID    string           `yaml:"devicedId"`
	Image        string           `yaml:"image"`
	ImageTag     string           `yaml:"imageTag"`
	ApiContainer dc.APIContainers `yaml:"apiContainer"`
}

// Stores state of container worker
type ContainerWorkerState struct {
	RunningContainers []RunningContainer `yaml:"runningContainers"`
}

type DevicedState struct {
	ContainerWorkerState ContainerWorkerState `yaml:"containerWorkerState"`
}

func (ds *DevicedState) stateFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (ds *DevicedState) WriteState(path string) bool {
	fmt.Printf("Writing state to %s\n", path)

	d, err := yaml.Marshal(&ds)
	if err != nil {
		fmt.Errorf("Error marshalling state: %v\n", err)
		return false
	}

	err = ioutil.WriteFile(path, d, 0644)
	return true
}

func (ds *DevicedState) CreateOrRead(path string) bool {
	if !ds.stateFileExists(path) {
		fmt.Printf("Writing empty state to %s\n", path)
		if !ds.WriteState(path) {
			fmt.Errorf("Unable to write empty state!\n")
			return false
		}
		return true
	}

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Errorf("Unable to read state at %s, %v, starting fresh", path, err)
		return true
	}

	err = yaml.Unmarshal(dat, &ds)
	if err != nil {
		fmt.Errorf("Unable to parse state at %s, %v, starting fresh", path, err)
		return true
	}

	fmt.Printf("Read state from %s\n", path)
	return true
}
