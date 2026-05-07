package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Feature struct {

	// ===== API =====
	HTTP struct {
		Enable bool `yaml:"enable"`
	} `yaml:"http"`

	// ===== Repository =====
	Repository struct {
		Enable bool `yaml:"enable"`
	} `yaml:"repository"`

	// ===== Earn Flow =====
	Earn struct {
		Publish struct {
			Enable    bool   `yaml:"enable"`
			Transport string `yaml:"transport"` // nats
		} `yaml:"publish"`
	} `yaml:"earn"`

	// ===== History Consumer =====
	History struct {
		Consume struct {
			Enable    bool   `yaml:"enable"`
			Transport string `yaml:"transport"` // nats
		} `yaml:"consume"`
	} `yaml:"history"`

	// ===== Redeem Flow =====
	Redeem struct {
		Publish struct {
			Enable    bool   `yaml:"enable"`
			Transport string `yaml:"transport"` // nats
		} `yaml:"publish"`

		Consume struct {
			Enable    bool   `yaml:"enable"`
			Transport string `yaml:"transport"` // nats
		} `yaml:"consume"`
	} `yaml:"redeem"`
}

func LoadFeature() *Feature {
	file := getEnv("FEATURE_FILE", "feature.yaml")

	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("cannot read feature file: %v", err)
	}

	var f Feature
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Fatalf("cannot parse feature yaml: %v", err)
	}

	return &f
}
