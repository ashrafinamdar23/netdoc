package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Name string `yaml:"name"`
	} `yaml:"app"`

	Logging LoggingConfig `yaml:"logging"`
}

type LoggingConfig struct {
	Level        string            `yaml:"level"`
	Format       string            `yaml:"format"`
	Output       string            `yaml:"output"`
	File         string            `yaml:"file"`
	ModuleLevels map[string]string `yaml:"module_levels"`
	Filters      LogFilters        `yaml:"filters"`
}

type LogFilters struct {
	RunID    string `yaml:"run_id"`
	ItemID   int64  `yaml:"item_id"`
	SystemID int64  `yaml:"system_id"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = "./config.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}

	if c.App.Name == "" {
		c.App.Name = "netdoc"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}
	if c.Logging.ModuleLevels == nil {
		c.Logging.ModuleLevels = map[string]string{}
	}

	if c.Logging.Output == "file" && c.Logging.File == "" {
		return Config{}, errors.New("logging.output=file requires logging.file")
	}

	return c, nil
}
