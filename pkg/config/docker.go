package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	dockerclient "github.com/docker/docker/client"
	"github.com/fuserobotics/devman/version"
)

type DockerClientTlsConfig struct {
	CaPemPath   string `yaml:"caPemPath,omitempty"`
	CertPemPath string `yaml:"certPemPath,omitempty"`
	KeyPemPath  string `yaml:"keyPemPath,omitempty"`
}

func (c *DockerClientTlsConfig) LoadTLSConfig() (*tls.Config, error) {
	conf := &tls.Config{}

	caDat, err := ioutil.ReadFile(c.CaPemPath)
	if err != nil {
		return nil, err
	}

	conf.ClientCAs = x509.NewCertPool()
	if !conf.ClientCAs.AppendCertsFromPEM(caDat) {
		return nil, errors.New("Unable to load client CA certs")
	}

	cert, err := tls.LoadX509KeyPair(c.CertPemPath, c.KeyPemPath)
	if err != nil {
		return nil, err
	}
	conf.Certificates = []tls.Certificate{cert}

	return conf, nil
}

func (c *DockerClientTlsConfig) Validate() bool {
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
	LoadFromEnvironment bool                  `yaml:"loadFromEnvironment,omitempty"`
	UseTls              bool                  `yaml:"useTls,omitempty"`
	TlsConfig           DockerClientTlsConfig `yaml:"tlsConfig,omitempty"`
	Endpoint            string                `yaml:"endpoint,omitempty"`
}

func (c *DockerClientConfig) FillWithDefaults() {
	if c.Endpoint == "" && !c.LoadFromEnvironment {
		c.Endpoint = "unix:///var/run/docker.sock"
		fmt.Printf("Using default endpoint of %s\n", c.Endpoint)
	}
}

func (c *DockerClientConfig) BuildClient() (*dockerclient.Client, error) {
	if c.Endpoint == "" {
		return nil, errors.New("No endpoint specified!")
	}

	if c.LoadFromEnvironment {
		return dockerclient.NewEnvClient()
	}

	tr := &http.Transport{}
	httpClient := &http.Client{Transport: tr}

	if c.UseTls {
		if !c.TlsConfig.Validate() {
			return nil, errors.New("Tls configuration failed to validate.")
		}

		// Attempt to load certs
		tconf, err := c.TlsConfig.LoadTLSConfig()
		if err != nil {
			return nil, err
		}

		tr.TLSClientConfig = tconf
	}

	return dockerclient.NewClient(c.Endpoint, fmt.Sprintf("devman-%s", version.Version), httpClient, map[string]string{})
}
