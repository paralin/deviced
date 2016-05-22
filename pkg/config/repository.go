package config

type RemoteRepository struct {
	Url         string              "url"
	PullPrefix  string              "pullPrefix"
	Username    string              "username,omitempty"
	Password    string              "password,omitempty"
	MetaHeaders map[string][]string "metaHeaders,omitempty"
	Insecure    bool                "insecure,omitempty"
}

func (r *RemoteRepository) RequiresAuth() bool {
	return r.Username != ""
}

// Later validate that it's a OK URL
func (r *RemoteRepository) Validate() bool {
	return r.Url != ""
}
