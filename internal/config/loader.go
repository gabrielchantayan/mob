package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// Load reads config from path, applying defaults for missing values
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadOrCreate loads config or creates default if missing
func LoadOrCreate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		return cfg, Save(path, cfg)
	}
	return Load(path)
}

// Save writes config to path
func Save(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}
