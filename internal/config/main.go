package config

import (
	"fmt"
	"log"
	"os"

	"github.com/go-playground/validator"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SubscriptionID string `yaml:"subscription" validate:"required"`
	ResourceGroup  string `yaml:"resource-group" validate:"required"`
	Vault          struct {
		Name      string `yaml:"name" validate:"required"`
		KeyPrefix string `yaml:"key-prefix" validate:"required"`
	} `yaml:"vault"`
	Bastion struct {
		Name   string `yaml:"name" validate:"required"`
		Server string `yaml:"server" validate:"required"`
	} `yaml:"bastion"`
	SSH struct {
		User string `yaml:"user" validate:"required"`
		Port int    `yaml:"port" validate:"min=0"`
		Dest string `yaml:"dest" validate:"required"`
	} `yaml:"ssh"`
	Cluster string `yaml:"cluster,omitempty"`
}

func LoadFromFile(configFile string) (*Config, error) {
	f, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("cannot open config file: %v", err)
	}
	defer f.Close()

	// load configuration
	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot decode config file: %v", err)
	}
	err = validator.New().Struct(cfg)
	if err != nil {
		return nil, fmt.Errorf("validation failed due to %v", err)
	}

	return &cfg, nil
}
