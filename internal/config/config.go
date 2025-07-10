package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	ServerPort   string `mapstructure:"SERVER_PORT"`
	DatabaseURL  string `mapstructure:"DATABASE_URL"`
	JWTSecret    string `mapstructure:"JWT_SECRET"`
	ClientOrigin string `mapstructure:"CLIENT_ORIGIN"`
	StripeAPIKey string
	// Add other configurations as needed
}

func LoadConfig(path string) (*Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env") // Name of config file (without extension)
	viper.SetConfigType("env")  // Or "dotenv" or "json", "yaml" etc.

	viper.AutomaticEnv() // Read in environment variables that match

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {
		// Handle errors reading the config file, but allow it if it's just "not found"
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found.")
		} else {
			return nil, err
		}
	}

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	cfg.StripeAPIKey = os.Getenv("STRIPE_API_KEY")

	return &cfg, nil
}
