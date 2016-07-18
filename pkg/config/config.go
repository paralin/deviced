package config

import (
	"fmt"
	"io/ioutil"
	"os"

	dc "github.com/fsouza/go-dockerclient"
	"github.com/go-yaml/yaml"
)

type DevicedConfig struct {
	ContainerConfig ContainerWorkerConfig      `json:"containerConfig"`
	ImageConfig     ImageWorkerConfig          `json:"imageConfig"`
	DockerConfig    DockerClientConfig         `json:"dockerConfig"`
	Repos           []*RemoteRepository        `json:"repos"`
	Containers      []*TargetContainer         `json:"containers"`
	Networks        []*dc.CreateNetworkOptions `json:"networks"`
}

func configFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (c *DevicedConfig) writeConfig(path string) bool {
	fmt.Printf("Writing config to %s\n", path)

	d, err := yaml.Marshal(&c)
	if err != nil {
		fmt.Printf("Error marshalling config: %v\n", err)
		return false
	}

	err = ioutil.WriteFile(path, d, 0644)
	return true
}

func (c *DevicedConfig) FillWithDefaults() {
	c.DockerConfig.FillWithDefaults()
	c.ImageConfig.FillWithDefaults()
}

func (c *DevicedConfig) ReadFrom(confPath string) bool {
	dat, err := ioutil.ReadFile(confPath)
	if err != nil {
		fmt.Printf("Unable to read config at %s, %v\n", confPath, err)
		return false
	}

	err = yaml.Unmarshal(dat, &c)
	if err != nil {
		fmt.Printf("Unable to parse config at %s, %v\n", confPath, err)
		return false
	}

	fmt.Printf("Read config from %s\n", confPath)
	c.FillWithDefaults()
	return true
}

func (c *DevicedConfig) CreateOrRead(confPath string) bool {
	if !configFileExists(confPath) {
		fmt.Printf("Writing default config to %s\n", confPath)
		c.FillWithDefaults()
		if !c.writeConfig(confPath) {
			fmt.Printf("Unable to write default config!\n")
			return false
		}
		return true
	}

	return c.ReadFrom(confPath)
}
