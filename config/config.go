package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string
	GRPC_PORT     string
	JWT_SECRET    string
	SUPABASE_URL  string
	SUPABASE_KEY  string
}

func LoadConfig() (*Config, error) {
	godotenv.Load("./config/.env_local")

	return &Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUsername: os.Getenv("NEO4J_USERNAME"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
		GRPC_PORT:     os.Getenv("GRPC_PORT"),
		JWT_SECRET:    os.Getenv("JWT_SECRET"),
		SUPABASE_URL:  os.Getenv("SUPABASE_URL"),
		SUPABASE_KEY:  os.Getenv("SUPABASE_KEY"),
	}, nil
}
