package zima

type Config struct {
	Types map[string]Type `json:"types"`
}

func (c *Config) Validate() error {
	panic("unimpl")
}

type Type struct {
	// Valid relations and the types of objects they can relate to
	Relations   []string            `json:"relations"`
	Permissions map[string][]string `json:"permissions"`
}

func NewConfigFromFile(bs []byte) (*Config, error) {
	// cfg.Validate()
	panic("unimpl")
}
