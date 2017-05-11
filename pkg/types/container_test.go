package types

import (
	"testing"
)

func TestConfigToAPI(t *testing.T) {
	conf := &Config{
		Hostname:     "test",
		Image:        "test/test",
		Cmd:          []string{"/do", "something"},
		Entrypoint:   []string{"/bin/bash"},
		AttachStdout: true,
		Labels: map[string]string{
			"key": "value",
		},
	}
	conf.ToAPI()
}
