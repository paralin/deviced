package config

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
