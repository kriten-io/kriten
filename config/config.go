package config

import (
	"os"
	"strconv"

	"k8s.io/client-go/kubernetes"
)

type LDAPConfig struct {
	BindUser string
	BindPass string
	FQDN     string
	Port     int
	BaseDN   string
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
	Port     int
	SSL      string
}

// type ESConfig struct {
// 	CloudID string
// 	APIKey  string
// 	Index   string
// }

type Config struct {
	Environment      string
	AdminsSecret     string
	RootSecret       string
	CommunityRelease bool
	DebugMode        bool // Not currently used
	LDAP             LDAPConfig
	Kube             KubeConfig
	JWT              JWTConfig
	DB               DBConfig
	// ElasticSearch    ESConfig
}

// New returns a new Config struct
func NewConfig(gitBranch string) Config {
	return Config{
		Environment:      getEnv("ENV", "development"),
		AdminsSecret:     getEnv("ADMINS_SECRET", "kriten-admins"),
		RootSecret:       getEnv("ROOT_SECRET", "kriten-root"),
		CommunityRelease: gitBranch == "community",
		DebugMode:        getEnvAsBool("DEBUG_MODE", true),
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
			JobsTTL:   getEnvAsInt("JOBS_TTL", 3600), // Default 1 hour
		},
		JWT: JWTConfig{
			Key:           []byte(getEnv("JWT_KEY", "")),
			ExpirySeconds: getEnvAsInt("JWT_EXPIRY_SECONDS", 3600), // Default 1 hour expiry
		},
		DB: DBConfig{
			Name:     getEnv("DB_NAME", ""),
			Host:     getEnv("DB_HOST", ""),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Port:     getEnvAsInt("DB_PORT", -1),
			SSL:      getEnv("DB_SSL", "disabled"),
		},
		// ElasticSearch: ESConfig{
		// 	CloudID: getEnv("ES_CLOUD_ID", ""),
		// 	APIKey:  getEnv("ES_API_KEY", ""),
		// 	Index:   getEnv("ES_INDEX", ""),
		// },
	}
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

// Helper to read an environment variable into a bool or return default value
func getEnvAsBool(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}

	return defaultVal
}
