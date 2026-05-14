package config

import "os"

type Config struct {
	AppPort string
	DataDir string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	return Config{
		AppPort: port,
		DataDir: dataDir,
	}
}
