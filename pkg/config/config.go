// Package config centralizes application settings derived from environment variables.
package config

import (
	"os"
)

// Config aggregates all operational parameters for the multi-agent system.
type Config struct {
	ProjectID     string
	Region        string
	LogLevel      string
	APIKey        string
	GCSBucketName string
	DatabaseURL   string
	DatabaseType  string
	StoreType     string
	PubSub        PubSubConfig
	Server        ServerConfig
	Models        ModelsConfig
}

// ModelsConfig specifies the AI model versions utilized by each agent in the pipeline.
type ModelsConfig struct {
	Aggregator     string
	Modeler        string
	Validator      string
	Reviewer       string
	Tagger         string
	Reporter       string
	VisualReporter string
}

// PubSubConfig maps agent stages to specific Google Cloud Pub/Sub topics and subscriptions.
type PubSubConfig struct {
	TopicScanRequests string
	SubScanRequests   string
	TopicAggregator   string
	SubAggregator     string
	TopicModeler      string
	SubModeler        string
	TopicValidator    string
	SubValidator      string
	TopicReviewer     string
	SubReviewer       string
	TopicTagger       string
	SubTagger         string
	TopicReporter     string
	SubReporter       string
	TopicMonitoring   string
	SubMonitoring     string
}

// ServerConfig stores settings related to the HTTP transport layer.
type ServerConfig struct {
	Port string
}

// Load populates the Config object from environment variables, using sane defaults for optional values.
func Load() *Config {
	projectID := os.Getenv("PROJECT_ID")
	return &Config{
		ProjectID:     projectID,
		Region:        os.Getenv("REGION"),
		LogLevel:      getEnv("LOG_LEVEL", "INFO"),
		APIKey:        os.Getenv("GEMINI_API_KEY"),
		GCSBucketName: getEnv("GCS_BUCKET_NAME", "cra-data-"+projectID),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		DatabaseType:  os.Getenv("DATABASE_TYPE"),
		StoreType:     getEnv("STORE_TYPE", "gcs"),
		PubSub: PubSubConfig{
			TopicScanRequests: getEnv("PUBSUB_TOPIC_SCAN_REQUESTS", "scan-requests"),
			SubScanRequests:   getEnv("PUBSUB_SUB_SCAN_REQUESTS", "scan-requests-sub"),
			TopicAggregator:   getEnv("PUBSUB_TOPIC_AGGREGATOR", "aggregator-tasks"),
			SubAggregator:     getEnv("PUBSUB_SUB_AGGREGATOR", "aggregator-tasks-sub"),
			TopicModeler:      getEnv("PUBSUB_TOPIC_MODELER", "modeler-tasks"),
			SubModeler:        getEnv("PUBSUB_SUB_MODELER", "modeler-tasks-sub"),
			TopicValidator:    getEnv("PUBSUB_TOPIC_VALIDATOR", "validator-tasks"),
			SubValidator:      getEnv("PUBSUB_SUB_VALIDATOR", "validator-tasks-sub"),
			TopicReviewer:     getEnv("PUBSUB_TOPIC_REVIEWER", "reviewer-tasks"),
			SubReviewer:       getEnv("PUBSUB_SUB_REVIEWER", "reviewer-tasks-sub"),
			TopicTagger:       getEnv("PUBSUB_TOPIC_TAGGER", "tagger-tasks"),
			SubTagger:         getEnv("PUBSUB_SUB_TAGGER", "tagger-tasks-sub"),
			TopicReporter:     getEnv("PUBSUB_TOPIC_REPORTER", "reporter-tasks"),
			SubReporter:       getEnv("PUBSUB_SUB_REPORTER", "reporter-tasks-sub"),
			TopicMonitoring:   getEnv("PUBSUB_TOPIC_MONITORING", "monitoring-events"),
			SubMonitoring:     getEnv("PUBSUB_SUB_MONITORING", "monitoring-events-sub"),
		},
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		Models: ModelsConfig{
			Aggregator:     getEnv("MODEL_AGGREGATOR", "gemini-3.1-flash-lite-preview"),
			Modeler:        getEnv("MODEL_MODELER", "gemini-3-pro-preview"),
			Validator:      getEnv("MODEL_VALIDATOR", "gemini-3-pro-preview"),
			Reviewer:       getEnv("MODEL_REVIEWER", "gemini-3-pro-preview"),
			Tagger:         getEnv("MODEL_TAGGER", "gemini-3.1-flash-lite-preview"),
			Reporter:       getEnv("MODEL_REPORTER", "gemini-3.1-flash-lite-preview"),
			VisualReporter: getEnv("MODEL_REPORTER", "gemini-3-pro-preview"),
		},
	}
}

// getEnv retrieves the value of an environment variable or returns a fallback if not set.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
