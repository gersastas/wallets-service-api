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
}

func findConfigFile() bool {
	_, err := os.Stat(envFileName)
	return err == nil
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
	envSetting := &EnvSetting{}

	helpString, err := envSetting.GetHelpString()
	if err != nil {
		logrus.Panicf("failed to get help string: %v", err)
	}

	logrus.Info(helpString)

	if findConfigFile() {
		if err := cleanenv.ReadConfig(envFileName, envSetting); err != nil {
			logrus.Panicf("failed to read env config: %v", err)
		}
	} else if err := cleanenv.ReadEnv(envSetting); err != nil {
		logrus.Panicf("failed to read env config: %v", err)
	}

	return &Config{env: envSetting}
}

func (c *Config) GetHTTPBindAddr() string {
	return c.env.HTTPBindAddr
}
