package pkg

import (
	"time"

	"github.com/spf13/viper"
)

// Config is the configuration for the application
type Config struct {
	DBDriver             string        `mapstructure:"DB_DRIVER"`
	DBSource             string        `mapstructure:"POSTGRES_URL"`
	HttpServerAddress    string        `mapstructure:"HTTP_SERVER_ADDRESS"`
	GrpcServerAddress    string        `mapstructure:"GRPC_SERVER_ADDRESS"`
	SymetricKey          string        `mapstructure:"SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`
}

// LoadConfig loads the configuration from the file
func LoadConfig(path string) (config Config, err error) {
	if path == "" {
		path = "." // Default to current directory if no path is provided
	}
	viper.AddConfigPath(path)
	viper.SetConfigName(".env") // Set the config name to ".env" without the extension
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
