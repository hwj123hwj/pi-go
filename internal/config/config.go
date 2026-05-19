package config

import "time"

type Config struct {
	Name        string
	Host        string
	Port        int
	DataDir     string
	SessionFile string
	MaxTurns    int
	Timeout     time.Duration
}

func Default() Config {
	return Config{
		Name:        "pi-go",
		Host:        "127.0.0.1",
		Port:        8080,
		DataDir:     "./data",
		SessionFile: "./data/session.jsonl",
		MaxTurns:    8,
		Timeout:     5 * time.Minute,
	}
}
