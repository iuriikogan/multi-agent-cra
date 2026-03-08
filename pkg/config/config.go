package config

import (
	"os"
)

type Config struct {
	ProjectID string
	Region    string
	LogLevel  string
	APIKey    string
	PubSub    PubSubConfig
	Server    ServerConfig
}

type PubSubConfig struct {
	TopicScanRequests string
	SubScanRequests   string
}

type ServerConfig struct {
	Port string
}

func Load() *Config {
	return &Config{
		ProjectID: os.Getenv("PROJECT_ID"),
		Region:    os.Getenv("REGION"),
		LogLevel:  getEnv("LOG_LEVEL", "INFO"),
		APIKey:    os.Getenv("GEMINI_API_KEY"),
		PubSub: PubSubConfig{
			TopicScanRequests: getEnv("PUBSUB_TOPIC_SCAN_REQUESTS", "scan-requests"),
			SubScanRequests:   getEnv("PUBSUB_SUB_SCAN_REQUESTS", "scan-requests-sub"),
		},
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
