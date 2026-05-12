package pool

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
)

type fileConfig struct {
	Pool filePoolConfig `yaml:"pool"`
}

type filePoolConfig struct {
	Tier        map[Tier]fileTierConfig `yaml:"tier"`
	DefaultTier Tier                    `yaml:"default-tier"`
}

type fileTierConfig struct {
	MaxIdleConns        *int    `yaml:"MaxIdleConns"`
	MaxIdleConnsPerHost *int    `yaml:"MaxIdleConnsPerHost"`
	MaxConnsPerHost     *int    `yaml:"MaxConnsPerHost"`
	IdleConnTimeout     *string `yaml:"IdleConnTimeout"`
}

// LoadConfig 설정 파일의 pool 설정을 기본 커넥션 풀 설정으로 반영합니다.
func LoadConfig(path string) error {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return err
	}

	configBody, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Errorf("read pool config file: %w", err)
	}

	var config fileConfig
	if err := yaml.UnmarshalWithOptions(configBody, &config, yaml.DisallowUnknownField()); err != nil {
		return fmt.Errorf("decode pool config file: %w", err)
	}

	poolConfigs, err := config.Pool.configs()
	if err != nil {
		return err
	}

	if err := Configure(poolConfigs, config.Pool.DefaultTier); err != nil {
		return fmt.Errorf("configure pool: %w", err)
	}

	slog.Info(
		logPoolConfigLoaded,
		logAttrDefaultTier,
		config.Pool.DefaultTier,
		logAttrPoolConfigs,
		poolConfigs,
	)

	return nil
}

func resolveConfigPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat pool config file: %w", err)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		candidate := filepath.Join(workingDirectory, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat pool config file: %w", err)
		}

		parent := filepath.Dir(workingDirectory)
		if parent == workingDirectory {
			break
		}
		workingDirectory = parent
	}

	return "", fmt.Errorf("%w: pool config file %q not found", ErrInvalidConfig, path)
}

func (c filePoolConfig) configs() (map[Tier]Config, error) {
	configs := make(map[Tier]Config, len(c.Tier))
	for tier, fileConfig := range c.Tier {
		poolConfig := defaultConfigs[tier]
		poolConfig.Tier = tier
		if fileConfig.MaxIdleConns != nil {
			poolConfig.MaxIdleConns = *fileConfig.MaxIdleConns
		}
		if fileConfig.MaxIdleConnsPerHost != nil {
			poolConfig.MaxIdleConnsPerHost = *fileConfig.MaxIdleConnsPerHost
		}
		if fileConfig.MaxConnsPerHost != nil {
			poolConfig.MaxConnsPerHost = *fileConfig.MaxConnsPerHost
		}
		if fileConfig.IdleConnTimeout != nil {
			idleConnTimeout, err := time.ParseDuration(*fileConfig.IdleConnTimeout)
			if err != nil {
				return nil, fmt.Errorf("parse %s idle connection timeout: %w", tier, err)
			}
			poolConfig.IdleConnTimeout = idleConnTimeout
		}

		configs[tier] = poolConfig
	}

	return configs, nil
}
