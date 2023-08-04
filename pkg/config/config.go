package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Config struct {
	TLint struct {
		Dir string `yaml:"dir" validate:"omitempty,dirpath"`
	} `bson:"yaml"`
	Config struct {
		File string `yaml:"file" validate:"omitempty,file"`
		URL  string `yaml:"url"  validate:"omitempty,url"`
	} `yaml:"config"`
	GolangCILint struct {
		Local bool   `yaml:"local"`
		File  string `yaml:"file" validate:"omitempty,file"`
		URL   string `yaml:"url"  validate:"omitempty,url"`
	} `yaml:"golangci-lint"`
	Override map[string]any `yaml:"override"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}

	cfg := &Config{}
	if err = yaml.NewDecoder(file).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	if err = validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	return cfg, err
}
