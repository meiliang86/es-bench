package config

import (
	"fmt"
	"time"

	"go.temporal.io/server/common/config"
	"go.temporal.io/server/common/dynamicconfig"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/metrics"
	espersistence "go.temporal.io/server/common/persistence/visibility/store/elasticsearch"
	"go.temporal.io/server/common/persistence/visibility/store/elasticsearch/client"
)

type (
	Config struct {
		Log           log.Config      `yaml:"log"`
		Metrics       *metrics.Config `yaml:"metrics"`
		Elasticsearch *client.Config  `yaml:"elasticsearch"`
		Processor     *Processor      `yaml:"processor"`
	}

	Processor struct {
		IndexerConcurrency int           `yaml:"indexConcurrency"`
		NumOfWorkers       int           `yaml:"numOfWorkers"`
		BulkActions        int           `yaml:"bulkActions"`
		BulkSize           int           `yaml:"bulkSize"`
		FlushInterval      time.Duration `yaml:"flushInterval"`
		AckTimeout         time.Duration `yaml:"ackTimeout"`
	}
)

// Helper function for loading configuration
func LoadConfig(env string, configDir string, zone string) (*Config, error) {
	cfg := Config{}
	err := config.Load(env, configDir, zone, &cfg)
	if err != nil {
		return nil, fmt.Errorf("config file corrupted: %w", err)
	}
	return &cfg, nil
}

func ProcessorConfig(cfg *Config) *espersistence.ProcessorConfig {
	return &espersistence.ProcessorConfig{
		IndexerConcurrency:       dynamicconfig.GetIntPropertyFn(cfg.Processor.IndexerConcurrency),
		ESProcessorNumOfWorkers:  dynamicconfig.GetIntPropertyFn(cfg.Processor.NumOfWorkers),
		ESProcessorBulkActions:   dynamicconfig.GetIntPropertyFn(cfg.Processor.BulkActions),
		ESProcessorBulkSize:      dynamicconfig.GetIntPropertyFn(cfg.Processor.BulkSize),
		ESProcessorFlushInterval: dynamicconfig.GetDurationPropertyFn(cfg.Processor.FlushInterval),
		ESProcessorAckTimeout:    dynamicconfig.GetDurationPropertyFn(cfg.Processor.AckTimeout),
	}
}
