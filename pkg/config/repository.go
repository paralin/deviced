package config

type RemoteRepository struct {
	Url         string              `json:"url"`
	PullPrefix  string              `json:"pullPrefix"`
	Username    string              `json:"username,omitempty"`
	Password    string              `json:"password,omitempty"`
	MetaHeaders map[string][]string `json:"metaHeaders,omitempty"`
	Insecure    bool                `json:"insecure,omitempty"`
}

func (r *RemoteRepository) RequiresAuth() bool {
	return r.Username != ""
}

// Later validate that it's a OK URL
func (r *RemoteRepository) Validate() bool {
	return r.Url != ""
}
