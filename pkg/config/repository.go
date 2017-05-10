package config

import (
	"encoding/base64"
	"fmt"
)

type RemoteRepository struct {
	Url         string              `yaml:"url"`
	PullPrefix  string              `yaml:"pullPrefix"`
	Username    string              `yaml:"username,omitempty"`
	Password    string              `yaml:"password,omitempty"`
	MetaHeaders map[string][]string `yaml:"metaHeaders,omitempty"`
	Insecure    bool                `yaml:"insecure,omitempty"`
}

func (r *RemoteRepository) RequiresAuth() bool {
	return r.Username != ""
}

// Later validate that it's a OK URL
func (r *RemoteRepository) Validate() bool {
	return r.Url != ""
}

func (r *RemoteRepository) BuildBase64Creds() string {
	credsStr := fmt.Sprintf("%s:%s", r.Username, r.Password)
	return base64.StdEncoding.EncodeToString([]byte(credsStr))
}
