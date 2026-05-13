package s3

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ray-d-song/mico/pkg/utils"
)

type extraConfig struct {
	Endpoint     string
	UsePathStyle bool
	Bucket       string
}

var (
	s3Client   *s3.Client
	clientOnce sync.Once
	clientErr  error
	s3Cfg      extraConfig
)

// parseExtraConfig extracts non-standard S3 options (endpoint & path style)
// from the INI file. Region, credentials are handled natively by the SDK.
func parseExtraConfig(path string) (extraConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return extraConfig{}, err
	}

	var cfg extraConfig
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if line == "[default]" {
			inSection = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "endpoint":
			cfg.Endpoint = value
		case "use_path_style":
			cfg.UsePathStyle = value == "true"
		case "bucket":
			cfg.Bucket = value
		}
	}

	return cfg, nil
}

// InitializeClient initializes the global S3 client singleton.
// Priority:
//  1. ~/.mico/s3.ini — SDK loads credentials/region from the file,
//     endpoint and use_path_style are parsed manually.
//  2. Fallback — AWS default credential chain (env vars, ~/.aws/, IAM, etc.).
//
// This should be called once at the start of the application.
func InitializeClient() error {
	clientOnce.Do(func() {
		ctx := context.Background()
		configPath := utils.GetS3ConfigPath()

		if configPath != "" {
			if _, err := os.Stat(configPath); err == nil {
				extra, err := parseExtraConfig(configPath)
				if err != nil {
					clientErr = fmt.Errorf("failed to read s3 config from %s: %w", configPath, err)
					return
				}
				s3Cfg = extra

				awsCfg, err := config.LoadDefaultConfig(ctx,
					config.WithSharedConfigFiles([]string{configPath}),
				)
				if err != nil {
					clientErr = fmt.Errorf("failed to load AWS config: %w", err)
					return
				}

				s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
					if extra.Endpoint != "" {
						o.BaseEndpoint = aws.String(extra.Endpoint)
					}
					o.UsePathStyle = extra.UsePathStyle
				})
				return
			}
		}

		awsCfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			clientErr = fmt.Errorf("failed to load AWS config: %w", err)
			return
		}

		s3Client = s3.NewFromConfig(awsCfg)
	})
	return clientErr
}

// GetClient returns the global S3 client instance.
// Panics if the client hasn't been initialized or initialization failed.
func GetClient() *s3.Client {
	if s3Client == nil {
		panic("S3 client not initialized.")
	}
	return s3Client
}

// GetConfig returns the S3 extra configuration parsed from s3.ini.
// Must be called after InitializeClient().
func GetConfig() extraConfig {
	return s3Cfg
}
