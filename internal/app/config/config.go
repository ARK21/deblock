package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	WsURL            string
	HttpUrl          string
	KafkaBrokers     []string
	KafkaTopic       string
	Confirmations    int
	ReorgDepth       int
	UsersFile        string
	HeadPollInterval time.Duration
	WSReconnectFloor time.Duration
	WSReconnectCeil  time.Duration
	CheckpointFile   string
	BootstrapBlocks  int
	HttpAddr         string
}

func Default() Config {
	return Config{
		KafkaTopic:       "tx_events",
		Confirmations:    3,
		ReorgDepth:       12,
		HeadPollInterval: 3 * time.Second,
		WSReconnectFloor: 1 * time.Second,
		WSReconnectCeil:  30 * time.Second,
	}
}

func Read() Config {
	cfg := Default()

	if usersFile, exists := os.LookupEnv("ADDRESSES_FILE"); exists {
		cfg.UsersFile = usersFile
	} else {
		log.Fatal("ADDRESSES_FILE environment variable not set")
	}
	if url, ok := os.LookupEnv("ETH_WS_URL"); ok {
		cfg.WsURL = url
	} else {
		log.Fatal("ETH_WS_URL env variable not set")
	}
	if url, ok := os.LookupEnv("ETH_HTTP_URL"); ok {
		cfg.HttpUrl = url
	} else {
		log.Fatal("ETH_HTTP_URL env variable not set")
	}
	if brokers, ok := os.LookupEnv("KAFKA_BROKERS"); ok {
		cfg.KafkaBrokers = []string{brokers}
	} else {
		log.Fatal("KAFKA_BROKERS env variable not set")
	}
	if kt, ok := os.LookupEnv("KAFKA_TOPIC"); ok {
		cfg.KafkaTopic = kt
	} else {
		log.Fatal("KAFKA_TOPIC env variable not set")
	}
	if cf, ok := os.LookupEnv("CHECKPOINT_FILE"); ok {
		cfg.CheckpointFile = cf
	} else {
		cfg.CheckpointFile = "./data/checkpoint.json"
	}
	if bb, ok := os.LookupEnv("BOOTSTRAP_BLOCKS"); ok {
		if bbInt, err := strconv.Atoi(bb); err == nil {
			cfg.BootstrapBlocks = bbInt
		} else {
			log.Fatalf("invalid BOOTSTRAP_BLOCKS value: %v", err)
		}
	} else {
		cfg.BootstrapBlocks = 0
	}
	if port, ok := os.LookupEnv("SERVICE_PORT"); ok {
		if _, err := strconv.Atoi(port); err == nil {
			cfg.HttpAddr = ":" + port
		} else {
			log.Fatalf("invalid SERVICE_PORT value: %v", err)
		}
	} else {
		cfg.HttpAddr = ":8080"
	}

	return cfg
}
