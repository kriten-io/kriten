package config

import (
	"log"
	"os"
	"strconv"

	"k8s.io/client-go/kubernetes"
)

const (
	JobsTTLDefault               = 3600
	JobsWorkerCountDefault       = 2
	JobsPollIntervalDefault      = 5
	SchedulerPollIntervalDefault = 10
	JWTExpirySecondsDefault      = 3600
	MCPStdoutLimitDefault        = 65536
	MCPMaxWaitSecondsDefault     = 30
	MCPPollIntervalMsDefault     = 2000
)

type LDAPConfig struct {
	BindUser string
	BindPass string
	FQDN     string
	BaseDN   string
	Port     int
}

type KubeConfig struct {
	Clientset *kubernetes.Clientset
	Namespace string
	JobsTTL   int
}

type JWTConfig struct {
	Key           []byte
	ExpirySeconds int
}

type DBConfig struct {
	Name     string
	Host     string
	User     string
	Password string
	SSL      string
	Port     int
}

type JobsConfig struct {
	WorkerCount  int
	PollInterval int
}

type SchedulerConfig struct {
	PollInterval int
}

type MCPConfig struct {
	Enabled        bool
	StdoutLimit    int
	MaxWaitSeconds int
	PollIntervalMs int
}

type Config struct {
	Environment  string
	RootPassword string
	APISecret    string
	SecretsKey   []byte
	LDAP         LDAPConfig
	Kube         KubeConfig
	JWT          JWTConfig
	DB           DBConfig
	Jobs         JobsConfig
	Scheduler    SchedulerConfig
	MCP          MCPConfig
	DebugMode    bool
}

// NewConfig returns a new Config struct.
func NewConfig(gitBranch string) Config {
	return Config{
		Environment:  getEnv("ENV", "development"),
		RootPassword: getEnv("ROOT_PASSWORD", "root"),
		APISecret:    getEnv("API_SECRET_KEY", "api-secret"),
		// SecretsKey is the master key used to encrypt runner secrets at rest.
		// It is hashed to 32 bytes before use, so any non-empty string works.
		SecretsKey: []byte(getEnv("SECRETS_ENCRYPTION_KEY", "kriten-dev-secrets-key")),
		DebugMode:  getEnvAsBool("DEBUG_MODE", true),
		LDAP: LDAPConfig{
			BindUser: getEnv("LDAP_BIND_USER", ""),
			BindPass: getEnv("LDAP_BIND_PASS", ""),
			FQDN:     getEnv("LDAP_FQDN", ""),
			Port:     getEnvAsInt("LDAP_PORT", -1),
			BaseDN:   getEnv("LDAP_BASE_DN", ""),
		},
		Kube: KubeConfig{
			Clientset: nil,
			Namespace: getEnv("NAMESPACE", "kriten"),
			JobsTTL:   getEnvAsInt("JOBS_TTL", JobsTTLDefault), // Default 1 hour
		},
		JWT: JWTConfig{
			Key:           []byte(getEnv("JWT_KEY", "")),
			ExpirySeconds: getEnvAsInt("JWT_EXPIRY_SECONDS", JWTExpirySecondsDefault), // Default 1 hour expiry
		},
		DB: DBConfig{
			Name:     getEnv("DB_NAME", ""),
			Host:     getEnv("DB_HOST", ""),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Port:     getEnvAsInt("DB_PORT", -1),
			SSL:      getEnv("DB_SSL", "disabled"),
		},
		Jobs: JobsConfig{
			WorkerCount:  getEnvAsInt("JOBS_WORKER_COUNT", JobsWorkerCountDefault),
			PollInterval: getEnvAsInt("JOBS_POLL_INTERVAL", JobsPollIntervalDefault),
		},
		Scheduler: SchedulerConfig{
			PollInterval: getEnvAsInt("SCHEDULER_POLL_INTERVAL", SchedulerPollIntervalDefault),
		},
		MCP: MCPConfig{
			Enabled:        getEnvAsBoolWarn("MCP_ENABLED", false),
			StdoutLimit:    getEnvAsPositiveIntWarn("MCP_STDOUT_LIMIT", MCPStdoutLimitDefault),
			MaxWaitSeconds: getEnvAsPositiveIntWarn("MCP_MAX_WAIT_SECONDS", MCPMaxWaitSecondsDefault),
			PollIntervalMs: getEnvAsPositiveIntWarn("MCP_POLL_INTERVAL_MS", MCPPollIntervalMsDefault),
		},
		// ElasticSearch: ESConfig{
		// 	CloudID: getEnv("ES_CLOUD_ID", ""),
		// 	APIKey:  getEnv("ES_API_KEY", ""),
		// 	Index:   getEnv("ES_INDEX", ""),
		// },
	}
}

// Simple helper function to read an environment or return a default value.
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value.
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

// Helper to read an environment variable into a bool or return default value.
func getEnvAsBool(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}

	return defaultVal
}

// getEnvAsBoolWarn reads a bool environment variable. An unparseable value is
// logged and the default is used.
func getEnvAsBoolWarn(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		log.Printf("config: %s=%q is not a valid bool, using default %t", name, valStr, defaultVal)
		return defaultVal
	}
	return val
}

// getEnvAsPositiveIntWarn reads a positive integer environment variable. An
// unparseable or non-positive value is logged and the default is used.
func getEnvAsPositiveIntWarn(name string, defaultVal int) int {
	valStr := getEnv(name, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		log.Printf("config: %s=%q must be a positive integer, using default %d", name, valStr, defaultVal)
		return defaultVal
	}
	return val
}
