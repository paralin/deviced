package config

import (
	"encoding/base64"
	"encoding/json"
	"github.com/docker/engine-api/types"
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
	auth := types.AuthConfig{
		Username: r.Username,
		Password: r.Password,
	}
	authBytes, _ := json.Marshal(auth)
	return base64.StdEncoding.EncodeToString(authBytes)
}
