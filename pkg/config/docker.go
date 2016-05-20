package config

import (
	"errors"
	"fmt"
	"os"

	dockerclient "github.com/fsouza/go-dockerclient"
)

type DockerClientTlsConfig struct {
	CaPemPath   string "caPemPath,omitempty"
	CertPemPath string "certPemPath,omitempty"
	KeyPemPath  string "keyPemPath,omitempty"
}

func (c *DockerClientTlsConfig) validate() bool {
	paths := [...]string{c.CaPemPath, c.CertPemPath, c.KeyPemPath}
	pathNames := [...]string{"ca pem", "cert pem", "key pem"}
	for idx, path := range paths {
		pathn := pathNames[idx]
		if path == "" {
			fmt.Errorf("No %s specified!\n", pathn)
			return false
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Errorf("%s at %s not found!\n", pathn, path)
			return false
		}
	}
	return true
}

type DockerClientConfig struct {
	LoadFromEnvironment bool                  "loadFromEnvironment,omitempty"
	UseTls              bool                  "useTls,omitempty"
	TlsConfig           DockerClientTlsConfig "tlsConfig,omitempty"
	Endpoint            string                "endpoint,omitempty"
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
