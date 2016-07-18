package config

import (
	"errors"
	"fmt"
	"os"

	dockerclient "github.com/fsouza/go-dockerclient"
)

type DockerClientTlsConfig struct {
	CaPemPath   string `json:"caPemPath,omitempty"`
	CertPemPath string `json:"certPemPath,omitempty"`
	KeyPemPath  string `json:"keyPemPath,omitempty"`
}

func (c *DockerClientTlsConfig) validate() bool {
	paths := [...]string{c.CaPemPath, c.CertPemPath, c.KeyPemPath}
	pathNames := [...]string{"ca pem", "cert pem", "key pem"}
	for idx, path := range paths {
		pathn := pathNames[idx]
		if path == "" {
			fmt.Printf("No %s specified!\n", pathn)
			return false
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("%s at %s not found!\n", pathn, path)
			return false
		}
	}
	return true
}

type DockerClientConfig struct {
	LoadFromEnvironment bool                  `json:"loadFromEnvironment,omitempty"`
	UseTls              bool                  `json:"useTls,omitempty"`
	TlsConfig           DockerClientTlsConfig `json:"tlsConfig,omitempty"`
	Endpoint            string                `json:"endpoint,omitempty"`
}

func (c *DockerClientConfig) FillWithDefaults() {
	if c.Endpoint == "" {
		c.Endpoint = "unix:///var/run/docker.sock"
		fmt.Printf("Using default endpoint of %s\n", c.Endpoint)
	}
}

func (c *DockerClientConfig) BuildClient() (*dockerclient.Client, error) {
	if c.Endpoint == "" {
		return nil, errors.New("No endpoint specified!")
	}
	if c.LoadFromEnvironment {
		return dockerclient.NewClientFromEnv()
	}
	if c.UseTls {
		if !c.TlsConfig.validate() {
			return nil, errors.New("Tls configuration failed to validate.")
		}
		return dockerclient.NewTLSClient(c.Endpoint, c.TlsConfig.CertPemPath, c.TlsConfig.KeyPemPath, c.TlsConfig.CaPemPath)
	}
	return dockerclient.NewClient(c.Endpoint)
}
