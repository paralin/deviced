package config

import (
	"fmt"
	"io/ioutil"
	"os"

	dcapi "github.com/docker/docker/api/types"
	"github.com/go-yaml/yaml"
)

type DevicedConfig struct {
	ContainerConfig ContainerWorkerConfig         `yaml:"containerConfig"`
	ImageConfig     ImageWorkerConfig             `yaml:"imageConfig"`
	DockerConfig    DockerClientConfig            `yaml:"dockerConfig"`
	Repos           []*RemoteRepository           `yaml:"repos"`
	Containers      []*TargetContainer            `yaml:"containers"`
	Networks        []*dcapi.NetworkCreateRequest `yaml:"networks"`
}

func ConfigFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (c *DevicedConfig) WriteConfig(path string) bool {
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

func (c *DevicedConfig) ReadFrom(confPath string) error {
	dat, err := ioutil.ReadFile(confPath)
	if err != nil {
		fmt.Printf("Unable to read config at %s, %v\n", confPath, err)
		return err
	}

	err = yaml.Unmarshal(dat, &c)
	if err != nil {
		fmt.Printf("Unable to parse config at %s, %v\n", confPath, err)
		return err
	}

	fmt.Printf("Read config from %s\n", confPath)
	c.FillWithDefaults()
	return nil
}

func (c *DevicedConfig) CreateOrRead(confPath string) bool {
	if !ConfigFileExists(confPath) {
		fmt.Printf("Writing default config to %s\n", confPath)
		c.FillWithDefaults()
		if !c.WriteConfig(confPath) {
			fmt.Printf("Unable to write default config!\n")
			return false
		}
		return true
	}

	return c.ReadFrom(confPath) == nil
}
