package config

import (
	"log"
	"os"
)

type Config struct {
	WsURL             string
	HttpUrl           string
	KafkaBrokers      []string
	KafkaTopic        string
	KafkaIdempotent   bool
	KafkaCompression  string
	KafkaRequiredAcks string
	Confirmations     int
	ReorgDepth        int
	UsersFile         string
}

func Default() Config {
	return Config{
		KafkaTopic:    "tx_events",
		Confirmations: 1,
		ReorgDepth:    12,
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

	return cfg
}
