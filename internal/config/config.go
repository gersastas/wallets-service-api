package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/sirupsen/logrus"
)

const envFileName = ".env"

type Config struct {
	env *EnvSetting
}

type EnvSetting struct {
	HTTPBindAddr string `env:"HTTP_BIND_ADDR" env-default:":8080" env-description:"HTTP server bind address"`
	DatabaseURL  string `env:"DATABASE_URL" env-default:"postgres://postgres:postgres@localhost:5432/wallet_db?sslmode=disable" env-description:"PostgreSQL connection string"`
}

func configFileExists() bool {
	_, err := os.Stat(envFileName)
	if err != nil {
		return false
	}
	return true
}

func (e *EnvSetting) GetHelpString() (string, error) {
	baseHeader := "options which can be set with env: "
	helpString, err := cleanenv.GetDescription(e, &baseHeader)
	if err != nil {
		return "", fmt.Errorf("failed to get help string: %w", err)
	}
	return helpString, nil
}

func New() *Config {
	var envSetting = new(EnvSetting)

	helpString, err := envSetting.GetHelpString()
	if err != nil {
		logrus.Panicf("failed to get help string: %v", err)
	}

	logrus.Info(helpString)

	if configFileExists() {
		if err := cleanenv.ReadConfig(envFileName, envSetting); err != nil {
			logrus.Panicf("failed to read env config: %v", err)
		}
		return &Config{env: envSetting}
	}

	if err := cleanenv.ReadEnv(envSetting); err != nil {
		logrus.Panicf("failed to read env config: %v", err)
	}

	return &Config{env: envSetting}
}

func (c *Config) GetHTTPBindAddr() string {
	return c.env.HTTPBindAddr
}

func (c *Config) GetDatabaseURL() string {
	return c.env.DatabaseURL
}
