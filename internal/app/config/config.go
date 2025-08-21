package config

import (
	"fmt"
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
	AddressesFile    string
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

	if confirmations, ok := os.LookupEnv("CONFIRMATIONS"); ok {
		if confInt, err := strconv.Atoi(confirmations); err == nil {
			cfg.Confirmations = confInt
		}
	}
	if reorgDepth, ok := os.LookupEnv("REORG_DEPTH"); ok {
		if reorgInt, err := strconv.Atoi(reorgDepth); err == nil {
			cfg.ReorgDepth = reorgInt
		}
	}
	if usersFile, exists := os.LookupEnv("ADDRESSES_FILE"); exists {
		cfg.AddressesFile = usersFile
	} else {
		cfg.AddressesFile = "./addresses.csv"
	}
	if headPollInterval, ok := os.LookupEnv("HEAD_POLL_INTERVAL"); ok {
		if hpi, err := time.ParseDuration(headPollInterval); err == nil {
			cfg.HeadPollInterval = hpi
		}
	}
	if wsReconnectFloor, ok := os.LookupEnv("WS_RECONNECT_FLOOR"); ok {
		if wrf, err := time.ParseDuration(wsReconnectFloor); err == nil {
			cfg.WSReconnectFloor = wrf
		}
	}
	if wsReconnectCeil, ok := os.LookupEnv("WS_RECONNECT_CEIL"); ok {
		if wrc, err := time.ParseDuration(wsReconnectCeil); err == nil {
			cfg.WSReconnectCeil = wrc
		}
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
	fmt.Printf("Watcher configs:\n")
	fmt.Printf("ETH_WS_URL: %s\n", cfg.WsURL)
	fmt.Printf("ETH_HTTP_URL: %s\n", cfg.HttpUrl)
	fmt.Printf("KAFKA_BROKERS: %s\n", cfg.KafkaBrokers)
	fmt.Printf("KAFKA_TOPIC: %s\n", cfg.KafkaTopic)
	fmt.Printf("CONFIRMATIONS: %d\n", cfg.Confirmations)
	fmt.Printf("REORG_DEPTH: %d\n", cfg.ReorgDepth)
	fmt.Printf("ADDRESSES_FILE: %s\n", cfg.AddressesFile)
	fmt.Printf("HEAD_POLL_INTERVAL: %s\n", cfg.HeadPollInterval)
	fmt.Printf("WS_RECONNECT_FLOOR: %s\n", cfg.WSReconnectFloor)
	fmt.Printf("WS_RECONNECT_CEIL: %s\n", cfg.WSReconnectCeil)
	fmt.Printf("CHECKPOINT_FILE: %s\n", cfg.CheckpointFile)
	fmt.Printf("BOOTSTRAP_BLOCKS: %d\n", cfg.BootstrapBlocks)
	fmt.Printf("SERVICE_PORT: %s\n", cfg.HttpAddr)

	return cfg
}
