package api

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/phamtuanha21092003/ecommerce-be/internal/config"
)

type (
	ApiConfig struct {
		*viper.Viper
		*config.App

		PostgresqlUri     string
		S3Bucket          string
		S3AccessKeyID     string
		S3SecretAccessKey string
		S3Endpoint        string
		StaticUrl         string
	}
)

var (
	apiConfig = &ApiConfig{
		Viper: viper.New(),
	}
)

func GetApiConfig() *ApiConfig {
	return apiConfig
}

func loadApiConfig() error {
	if err := config.LoadAppCfg(); err != nil {
		return fmt.Errorf("load app config: %w", err)
	}
	apiConfig.App = config.GetAppCfg()

	apiConfig.PostgresqlUri = os.Getenv("POSTGRESQL_URI")
	if apiConfig.PostgresqlUri == "" {
		return errors.New("POSTGRESQL_URI is required")
	}

	apiConfig.S3Bucket = os.Getenv("S3_BUCKET")
	if apiConfig.S3Bucket == "" {
		return errors.New("S3_BUCKET is required")
	}

	apiConfig.S3AccessKeyID = os.Getenv("S3_ACCESS_KEY_ID")
	if apiConfig.S3AccessKeyID == "" {
		return errors.New("S3_ACCESS_KEY_ID is required")
	}

	apiConfig.S3SecretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")
	if apiConfig.S3SecretAccessKey == "" {
		return errors.New("S3_SECRET_ACCESS_KEY is required")
	}

	apiConfig.S3Endpoint = os.Getenv("S3_ENDPOINT")
	if apiConfig.S3Endpoint == "" {
		return errors.New("S3_ENDPOINT is required")
	}

	apiConfig.StaticUrl = os.Getenv("STATIC_URL")
	if apiConfig.StaticUrl == "" {
		return errors.New("STATIC_URL is required")
	}

	return nil
}
