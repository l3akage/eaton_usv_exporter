package config

import (
	"io"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for the exporter
type Config struct {
	Targets   []string `yaml:"targets"`
	Community string   `yaml:"community"`
}

func New() *Config {
	c := &Config{}
	return c
}

// Load loads a config from reader
func Load(reader io.Reader) (*Config, error) {
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	c := New()
	err = yaml.Unmarshal(b, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
