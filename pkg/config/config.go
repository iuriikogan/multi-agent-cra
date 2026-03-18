// Package config centralizes all operational settings derived from environment variables.
//
// Rationale: Decoupling configuration from logic ensures the Multi-Agent system remains
// portable across environments (local, dev, prod). This package enforces defaults and
// centralizes all project-wide constants to maintain visibility and security.
package config

import (
	"os"
	"strings"
)

// Config aggregates all operational parameters for the multi-agent system.
type Config struct {
	ProjectID     string       // The Google Cloud Project ID for APIs, Tracing, and Storage.
	Region        string       // The Google Cloud region for services (e.g., "europe-west1").
	LogLevel      string       // The structured log level (DEBUG, INFO, WARN, ERROR).
	APIKey        string       // The API key for Gemini LLM access.
	GCSBucketName string       // The cloud storage bucket name for compliance artifacts.
	DatabaseURL   string       // The connection string for Cloud SQL (MySQL) or a local SQLite file path.
	DatabaseType  string       // The database engine used (e.g., "cloudsql" or "sqlite").
	StoreType     string       // The type of storage implementation used ("gcs", "sqlite").
	PubSub        PubSubConfig // Specific topic and subscription names for the event-driven pipeline.
	Server        ServerConfig // HTTP configuration settings.
	Models        ModelsConfig // AI Model version mapping for individual specialized agents.
}

// ModelsConfig specifies the specific GenAI model versions assigned to each agent's role.
type ModelsConfig struct {
	Aggregator     string // The model for asset discovery and enumeration.
	Modeler        string // The model for mapping assets to regulatory frameworks.
	Validator      string // The model for evaluating compliance requirements.
	Reviewer       string // The model for peer-reviewing validation results.
	Tagger         string // The model for applying governance tags.
	Reporter       string // The model for generating textual compliance reports.
	VisualReporter string // The model for generating visual dashboards (e.g., gemini-2.5-flash-image).
}

// PubSubConfig defines the mapping of event stages to Google Cloud Pub/Sub resources.
type PubSubConfig struct {
	TopicScanRequests string // Topic for initial assessment triggers.
	SubScanRequests   string // Subscription for the aggregator agent.
	TopicAggregator   string // Topic for individual resource assessment tasks.
	SubAggregator     string // Subscription for the modeler agent.
	TopicModeler      string // Topic for model assessment outcomes.
	SubModeler        string // Subscription for the validator agent.
	TopicValidator    string // Topic for validation findings.
	SubValidator      string // Subscription for the reviewer agent.
	TopicReviewer     string // Topic for peer-reviewed outcomes.
	SubReviewer       string // Subscription for the tagging agent.
	TopicTagger       string // Topic for completion events.
	SubTagger         string // Subscription for the final reporting engine.
	TopicReporter     string // Topic for final compliance reports.
	SubReporter       string // Subscription for result persistence.
	TopicMonitoring   string // Topic for internal agent telemetry (monitoring events).
	SubMonitoring     string // Subscription for live dashboard updates.
}

// ServerConfig stores settings related to the HTTP transport layer and API server.
type ServerConfig struct {
	Port string // Port number the API server listens on (e.g., "8080").
}

// Load reads and parses all necessary environment variables to construct the Config object.
// It provides sane defaults where applicable to simplify deployment.
func Load() *Config {
	projectID := os.Getenv("PROJECT_ID")
	return &Config{
		ProjectID:     projectID,
		Region:        os.Getenv("REGION"),
		LogLevel:      strings.ToUpper(getEnv("LOG_LEVEL", "INFO")),
		APIKey:        os.Getenv("GEMINI_API_KEY"),
		GCSBucketName: getEnv("GCS_BUCKET_NAME", "compliance-data-"+projectID),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		DatabaseType:  os.Getenv("DATABASE_TYPE"),
		StoreType:     getEnv("STORE_TYPE", "sqlite"),
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
			Modeler:        getEnv("MODEL_MODELER", "gemini-3.1-flash-lite-preview"),
			Validator:      getEnv("MODEL_VALIDATOR", "gemini-3.1-flash-lite-preview"),
			Reviewer:       getEnv("MODEL_REVIEWER", "gemini-3.1-flash-lite-preview"),
			Tagger:         getEnv("MODEL_TAGGER", "gemini-3.1-flash-lite-preview"),
			Reporter:       getEnv("MODEL_REPORTER", "gemini-3.1-flash-lite-preview"),
			VisualReporter: getEnv("MODEL_REPORTER", "gemini-2.5-flash-image"),
		},
	}
}

// getEnv retrieves the value of an environment variable or returns a fallback if it is not set.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
