package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

type ConfigSchema struct {
	Database struct {
		Name     string `yaml:"name"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"db"`
	Backend struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"backend"`
	Logs struct {
		Level     string `yaml:"level"`
		SentrySDK string `yaml:"sentry_sdk"`
	} `yaml:"logs"`
}

var AppConfig ConfigSchema

func LoadConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, &AppConfig)
	return err
}
