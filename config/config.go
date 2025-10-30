package config

import (
	"os"
)

type Config struct {
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string
	GRPC_PORT     string
}

func LoadConfig() (*Config, error) {
	//.env 파일 로드 (개발 환경에서만 필요)
	// if os.Getenv("NEO4J_URI") == "" {
	// 	err := godotenv.Load("../../config/.env")
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	return &Config{
		Neo4jURI:      os.Getenv("NEO4J_URI"),
		Neo4jUsername: os.Getenv("NEO4J_USERNAME"),
		Neo4jPassword: os.Getenv("NEO4J_PASSWORD"),
		GRPC_PORT:     os.Getenv("GRPC_PORT"),
	}, nil
}
