package config

import (
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LogLevel      string `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	SearchAddress string `yaml:"search_address" env:"SEARCH_ADDRESS" env-default:"localhost:83"`
	DBAddress     string `yaml:"db_address" env:"DB_ADDRESS" env-default:"localhost:1234"`
	WordsAddress  string `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:82"`
	BrokerAddress string `yaml:"broker_address" env:"BROKER_ADDRESS" env-default:"nats://localhost:4222"`
	TTL           string `yaml:"TTL" env:"INDEX_TTL" env-default:"20s"`
}

func MustLoad(path string) Config {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		log.Fatalf("cannot read config %q: %s", path, err)
	}
	return cfg
}
