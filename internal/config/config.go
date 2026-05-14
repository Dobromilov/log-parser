package config

import "os"

type Config struct {
	AppPort       string
	DataDir       string
	DatabaseURL   string
	MigrationsDir string
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

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}

	return Config{
		AppPort:       port,
		DataDir:       dataDir,
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		MigrationsDir: migrationsDir,
	}
}
