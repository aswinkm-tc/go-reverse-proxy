package backend

import (
	"net/url"

	"gopkg.in/yaml.v3"
)

type Backend struct {
	// Name Optional name for the backend
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Address of the backend service
	Address *url.URL `json:"address" yaml:"address"`
}

func (b *Backend) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := url.Parse(raw["address"])
	if err != nil {
		return err
	}
	b.Address = parsed
	if v, ok := raw["name"]; ok {
		// If name is provided, set it
		b.Name = v
	} else {
		// If name is not provided, use the host part of the address as the name
		b.Name = parsed.Host
	}
	return nil
}
