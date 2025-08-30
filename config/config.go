package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Kafka      KafkaConfig
	Postgres   PostgresConfig
	Processing ProcessingConfig
	Vectorizer VectorizerConfig
	OpenAI     OpenAIConfig
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
}

type PostgresConfig struct {
	DSN string
}

type ProcessingConfig struct {
	BatchSize       int
	TimeoutPerBatch time.Duration
}

type VectorizerConfig struct {
	Model           string
	BatchSize       int
	TimeoutPerBatch time.Duration
	MaxVectorLength int
}

type OpenAIConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	MaxRetries int
	Timeout    time.Duration
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: No config file found, using defaults: %v\n", err)
	}

	viper.BindEnv("OPENAI_API_KEY")
	viper.BindEnv("PG_DSN")

	var config = &Config{
		Kafka: KafkaConfig{
			Brokers: viper.GetStringSlice("kafka.brokers"),
			GroupID: viper.GetString("kafka.group_id"),
		},
		Postgres: PostgresConfig{
			DSN: viper.GetString("PG_DSN"),
		},
		Processing: ProcessingConfig{
			BatchSize:       viper.GetInt("processing.batch_size"),
			TimeoutPerBatch: viper.GetDuration("processing.timeout_seconds"),
		},
		Vectorizer: VectorizerConfig{
			Model:           viper.GetString("vectorizer.model"),
			BatchSize:       viper.GetInt("vectorizer.batch_size"),
			MaxVectorLength: viper.GetInt("vectorizer.max_vector_length"),
			TimeoutPerBatch: viper.GetDuration("vectorizer.timeout_seconds"),
		},
		OpenAI: OpenAIConfig{
			APIKey:     viper.GetString("OPENAI_API_KEY"),
			BaseURL:    viper.GetString("openai.base_url"),
			Model:      viper.GetString("openai.model"),
			MaxRetries: viper.GetInt("openai.max_retries"),
			Timeout:    viper.GetDuration("openai.timeout_seconds"),
		},
	}

	return config, nil
}
