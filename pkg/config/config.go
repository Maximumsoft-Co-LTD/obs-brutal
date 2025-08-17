package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type (
	ObsConfig struct {
		AppSrv     AppService `yaml:"service" mapstructure:",squash"`
		Database   Database   `yaml:"database" mapstructure:",squash"`
		Log        Logger     `yaml:"logger" mapstructure:",squash"`
		Metrics    `yaml:"metrics" mapstructure:",squash"`
		Otel       `yaml:"otel" mapstructure:",squash"`
		Promtail   `yaml:"promtail" mapstructure:",squash"`
		Prometheus `yaml:"prometheus" mapstructure:",squash"`
		Loki       `yaml:"loki" mapstructure:",squash"`
		Masking    `yaml:"masking" mapstructure:",squash"`
		AMQP       `yaml:"amqp" mapstructure:",squash"`
	}

	AppService struct {
		SERVICE     string `yaml:"service_name" mapstructure:"SERVICE" default:"POT_SERVICE"`
		ENVIRONMENT string `yaml:"environment" mapstructure:"ENVIRONMENT" default:"development"`
		PORT        string `yaml:"port" mapstructure:"PORT" default:":8080"`
		GIN_MODE    string `yaml:"gin_mode" mapstructure:"GIN_MODE" default:"release"`
	}
	//#region Database
	Database struct {
		MongoDB `yaml:"mongodb" mapstructure:",squash"`
		Redis   `yaml:"redis" mapstructure:",squash"`
	}

	MongoDB struct {
		ENABLED  bool   `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		ADDR     string `log:"addr,sensitive=true" yaml:"addr" mapstructure:"ADDR" default:"localhost:27017"`
		PASSWORD string `log:"password,sensitive=true" yaml:"password" mapstructure:"PASSWORD" default:""`
		DB       int    `log:"db" yaml:"db" mapstructure:"DB" default:"0"`
	}

	Redis struct {
		ENABLED  bool   `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		ADDR     string `log:"addr" yaml:"addr" mapstructure:"ADDR" default:"localhost:6379"`
		PASSWORD string `log:"password,sensitive=true" yaml:"password" mapstructure:"PASSWORD" default:""`
		DB       int    `log:"db" yaml:"db" mapstructure:"DB" default:"0"`
	}
	//#endregion

	//#region AMQP
	AMQP struct {
		ENABLED bool   `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		URL     string `log:"url,sensitive=true" yaml:"url" mapstructure:"URL" default:"amqp://guest:guest@localhost:5672/"`
	}
	//#endregion

	//#region Logger
	Logger struct {
		LEVEL   string `yaml:"log_level" mapstructure:"LEVEL" default:"info"`
		FILE    string `yaml:"log_file" mapstructure:"FILE" default:""`
		BACKEND string `yaml:"backend" mapstructure:"BACKEND" default:"zap"`
		FORMAT  string `yaml:"format" mapstructure:"FORMAT" default:"json"`
	}
	//#endregion

	//#region Metrics
	Metrics struct {
		PORT     int    `yaml:"port" mapstructure:"PORT" default:"9090"`
		PATH     string `yaml:"path" mapstructure:"PATH" default:"/metrics"`
		ENABLED  bool   `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		ENDPOINT string `yaml:"endpoint" mapstructure:"ENDPOINT" default:"localhost:9464"`
		INSECURE bool   `yaml:"insecure" mapstructure:"INSECURE" default:"true"`
	}
	//#endregion

	//#region Otel
	Otel struct {
		ENABLED       bool    `yaml:"enabled" mapstructure:"ENABLED" default:"false"`
		ENDPOINT      string  `yaml:"endpoint" mapstructure:"ENDPOINT" default:"localhost:4317"`
		INSECURE      bool    `yaml:"insecure" mapstructure:"INSECURE" default:"true"`
		SERVICE_NAME  string  `yaml:"service_name" mapstructure:"SERVICE_NAME" default:"POT_SERVICE"`
		ENVIRONMENT   string  `yaml:"environment" mapstructure:"ENVIRONMENT" default:"development"`
		SAMPLER_RATIO float64 `yaml:"sampler_ratio" mapstructure:"SAMPLER_RATIO" default:"1.0"`
	}
	//#endregion

	//#region Promtail
	Promtail struct {
		DIR_FILE  string `yaml:"dir_file" mapstructure:"DIR_FILE" default:"logs"`
		PATH_FILE string `yaml:"path_file" mapstructure:"PATH_FILE" default:"app.log"`
	}
	//#endregion

	//#region Prometheus
	Prometheus struct {
		ENABLED        bool   `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		PATH           string `yaml:"path" mapstructure:"PATH" default:"/metrics"`
		STANDALONE     bool   `yaml:"standalone" mapstructure:"STANDALONE" default:"true"`
		LISTEN_ADDRESS string `yaml:"listen_address" mapstructure:"LISTEN_ADDRESS" default:":9091"`
	}
	//#endregion

	//#region Loki
	Loki struct {
		ENABLED bool              `yaml:"enabled" mapstructure:"ENABLED" default:"false"`
		URL     string            `yaml:"url" mapstructure:"URL" default:"http://localhost:3100"`
		LABELS  map[string]string `yaml:"labels" mapstructure:"LABELS"`
	}
	//#endregion

	//#region Masking
	Masking struct {
		ENABLED  bool              `yaml:"enabled" mapstructure:"ENABLED" default:"true"`
		FIELDS   []string          `yaml:"fields" mapstructure:"FIELDS"`
		PATTERNS map[string]string `yaml:"patterns" mapstructure:"PATTERNS"`
	}
	//#endregion
)

// Load loads configuration from file and environment variables
func Load(configPath string) (*ObsConfig, error) {
	vpp := viper.New()
	vpp.SetEnvPrefix("")
	vpp.AutomaticEnv()
	vpp.AllowEmptyEnv(true)
	cfg := &ObsConfig{}
	// Step 1: Set default values
	if err := setDefaults(vpp, cfg); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	// Step 2: Load from YAML file if exists
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			// Log warning but continue - file is optional
			fmt.Printf("Warning: Could not load config file %s: %v\n", configPath, err)
		}
	}

	// Step 3: Override with environment variables
	if err := loadFromEnv(vpp, cfg); err != nil {
		return nil, fmt.Errorf("failed to load env vars: %w", err)
	}

	// Step 4: Set defaults for complex fields if not set
	if cfg.Masking.FIELDS == nil {
		cfg.Masking.FIELDS = []string{
			"password", "api_key", "credit_card",
			"access_token", "refresh_token", "secret",
		}
	}

	if cfg.Masking.PATTERNS == nil {
		cfg.Masking.PATTERNS = map[string]string{}
	}

	if cfg.Loki.LABELS == nil {
		cfg.Loki.LABELS = map[string]string{
			"app": cfg.AppSrv.SERVICE,
			"env": cfg.AppSrv.ENVIRONMENT,
		}
	}

	return cfg, nil
}

// LoadFromFile loads configuration from YAML file
func loadFromFile(cfg *ObsConfig, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// fmt.Println("data----------->>>", string(data))
	// Expand environment variables in YAML
	expanded := os.ExpandEnv(string(data))
	// log.Println("expanded----------->>>", expanded)

	return yaml.Unmarshal([]byte(expanded), cfg)
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(vpp *viper.Viper, cfg interface{}) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	return processFields(vpp, v, t, "")
}

// processFields recursively processes struct fields
func processFields(vpp *viper.Viper, v reflect.Value, t reflect.Type, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanSet() {
			continue
		}

		// Check for inline tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == ",inline" || strings.Contains(yamlTag, ",inline") {
			// Process inline struct fields directly
			if field.Type.Kind() == reflect.Struct {
				if err := processFields(vpp, fieldValue, field.Type, prefix); err != nil {
					return err
				}
			}
			continue
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			// For non-inline structs, process recursively
			if err := processFields(vpp, fieldValue, field.Type, prefix); err != nil {
				return err
			}
			continue
		}

		// Handle slices and maps
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Map {
			// Skip complex types for env loading
			continue
		}

		// Get env tag
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Check if environment variable is set
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set the value based on field type
		if err := setFieldValue(fieldValue, field.Type, envValue); err != nil {
			return fmt.Errorf("failed to set %s: %w", field.Name, err)
		}
	}

	return nil
}

// setDefaults sets default values from struct tags
func setDefaults(vpp *viper.Viper, cfg interface{}) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	return setDefaultFields(vpp, v, t)
}

// setDefaultFields recursively sets default values
func setDefaultFields(vpp *viper.Viper, v reflect.Value, t reflect.Type) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanSet() {
			continue
		}

		// Check for inline tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == ",inline" || strings.Contains(yamlTag, ",inline") {
			// Process inline struct fields directly
			if field.Type.Kind() == reflect.Struct {
				if err := setDefaultFields(vpp, fieldValue, field.Type); err != nil {
					return err
				}
			}
			continue
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			if err := setDefaultFields(vpp, fieldValue, field.Type); err != nil {
				return err
			}
			continue
		}

		// Get default tag
		defaultTag := field.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		// Set the default value
		if err := setFieldValue(fieldValue, field.Type, defaultTag); err != nil {
			return fmt.Errorf("failed to set default for %s: %w", field.Name, err)
		}
		vpp.SetDefault(field.Name, defaultTag)
	}

	return nil
}

// setFieldValue sets a field value based on its type
func setFieldValue(field reflect.Value, fieldType reflect.Type, value string) error {
	switch fieldType.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intVal)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			// Handle common bool representations
			value = strings.ToLower(value)
			boolVal = value == "true" || value == "yes" || value == "1" || value == "on"
		}
		field.SetBool(boolVal)

	default:
		return fmt.Errorf("unsupported field type: %s", fieldType.Kind())
	}

	return nil
}

// String returns a string representation of the config
func (c *ObsConfig) String() string {
	data, _ := yaml.Marshal(c)
	return string(data)
}
