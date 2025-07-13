package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	ServerPort              string `mapstructure:"SERVER_PORT"`
	DatabaseURL             string `mapstructure:"DATABASE_URL"`
	JWTSecret               string `mapstructure:"JWT_SECRET"`
	ClientOrigin            string `mapstructure:"CLIENT_ORIGIN"`
	GoogleOAuthClientID     string `mapstructure:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecret string `mapstructure:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleOAuthRedirectURL  string `mapstructure:"GOOGLE_OAUTH_REDIRECT_URL"`
	AWSRegion               string `mapstructure:"AWS_REGION"`
	AWSAccessKeyID          string `mapstructure:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey      string `mapstructure:"AWS_SECRET_ACCESS_KEY"`
	EmailFromAddress        string `mapstructure:"EMAIL_FROM_ADDRESS"`
	GoogleMapsAPIKey        string `mapstructure:"GOOGLE_MAPS_API_KEY"`
	StripeAPIKey            string `mapstructure:"STRIPE_API_KEY"`
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
