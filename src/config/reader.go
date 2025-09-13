package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type Config struct {
	Databases struct {
		Master   DBConfig   `yaml:"master"`
		Replicas []DBConfig `yaml:"replicas"`
	} `yaml:"db"`
	Redis        RedisConfig `yaml:"redis"`
	RedisDialogs RedisConfig `yaml:"redis_dialogs"`
	Backend      struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"backend"`
	RabbitMQ struct {
		URL string `yaml:"url"`
	}
	Logs struct {
		Level     string `yaml:"level"`
		SentrySDK string `yaml:"sentry_sdk"`
	} `yaml:"logs"`
	ShardCount int `yaml:"shard_count"`
}

var AppConfig *Config

func LoadConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, &AppConfig)
	return err
}
