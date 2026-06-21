package config

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
	Shared *filePoolConfigValue       `yaml:"shared"`
	Tier   map[Tier]filePoolConfigValue `yaml:"tier"`
}

type filePoolConfigValue struct {
	MaxIdleConns          *int    `yaml:"MaxIdleConns"`
	MaxIdleConnsPerHost   *int    `yaml:"MaxIdleConnsPerHost"`
	MaxConnsPerHost       *int    `yaml:"MaxConnsPerHost"`
	IdleConnTimeout       *string `yaml:"IdleConnTimeout"`
	ResponseHeaderTimeout *string `yaml:"ResponseHeaderTimeout"`
	TLSHandshakeTimeout   *string `yaml:"TLSHandshakeTimeout"`
	ExpectContinueTimeout *string `yaml:"ExpectContinueTimeout"`
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

	sharedConfig, tierConfigs, err := config.Pool.configs()
	if err != nil {
		return err
	}

	if err := Configure(sharedConfig, tierConfigs); err != nil {
		return fmt.Errorf("configure pool: %w", err)
	}

	slog.Info(
		logPoolConfigLoaded,
		logAttrSharedConfig,
		sharedConfig,
		logAttrPoolConfigs,
		tierConfigs,
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

func (c filePoolConfig) configs() (Config, map[Tier]Config, error) {
	if c.Shared == nil {
		return Config{}, nil, fmt.Errorf("%w: shared pool config is required", ErrInvalidConfig)
	}
	if len(c.Tier) == 0 {
		return Config{}, nil, fmt.Errorf("%w: tier pool configs are required", ErrInvalidConfig)
	}

	sharedConfig, err := c.Shared.config("shared")
	if err != nil {
		return Config{}, nil, err
	}

	configs := make(map[Tier]Config, len(c.Tier))
	for tier, fileConfig := range c.Tier {
		poolConfig, err := fileConfig.config(string(tier))
		if err != nil {
			return Config{}, nil, err
		}
		poolConfig.Tier = tier

		configs[tier] = poolConfig
	}

	return sharedConfig, configs, nil
}

func (c filePoolConfigValue) config(name string) (Config, error) {
	if c.MaxIdleConns == nil {
		return Config{}, fmt.Errorf("%w: %s MaxIdleConns is required", ErrInvalidConfig, name)
	}
	if c.MaxIdleConnsPerHost == nil {
		return Config{}, fmt.Errorf("%w: %s MaxIdleConnsPerHost is required", ErrInvalidConfig, name)
	}
	if c.MaxConnsPerHost == nil {
		return Config{}, fmt.Errorf("%w: %s MaxConnsPerHost is required", ErrInvalidConfig, name)
	}
	if c.IdleConnTimeout == nil {
		return Config{}, fmt.Errorf("%w: %s IdleConnTimeout is required", ErrInvalidConfig, name)
	}

	idleConnTimeout, err := time.ParseDuration(*c.IdleConnTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse %s idle connection timeout: %w", name, err)
	}

	poolConfig := Config{
		MaxIdleConns:          *c.MaxIdleConns,
		MaxIdleConnsPerHost:   *c.MaxIdleConnsPerHost,
		MaxConnsPerHost:       *c.MaxConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
	}

	if c.ResponseHeaderTimeout != nil {
		responseHeaderTimeout, err := time.ParseDuration(*c.ResponseHeaderTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s response header timeout: %w", name, err)
		}
		poolConfig.ResponseHeaderTimeout = responseHeaderTimeout
	}
	if c.TLSHandshakeTimeout != nil {
		tlsHandshakeTimeout, err := time.ParseDuration(*c.TLSHandshakeTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s tls handshake timeout: %w", name, err)
		}
		poolConfig.TLSHandshakeTimeout = tlsHandshakeTimeout
	}
	if c.ExpectContinueTimeout != nil {
		expectContinueTimeout, err := time.ParseDuration(*c.ExpectContinueTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s expect continue timeout: %w", name, err)
		}
		poolConfig.ExpectContinueTimeout = expectContinueTimeout
	}

	return poolConfig, nil
}
