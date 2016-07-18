package config

import "fmt"

type ImageWorkerConfig struct {
	RecheckPeriod int `yaml:"recheckPeriod"`
}

func (c *ImageWorkerConfig) FillWithDefaults() {
	if c.RecheckPeriod == 0 {
		c.RecheckPeriod = 60
		fmt.Printf("Using default recheck period of %d\n", c.RecheckPeriod)
	}
}
