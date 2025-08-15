package config

import "os"

type Config struct {
	RPCURLs           []string
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
		Confirmations: 3,
		ReorgDepth:    12,
	}
}

func Read() Config {
	cfg := Default()

	if usersFile, exists := os.LookupEnv("USERS_FILE"); exists {
		cfg.UsersFile = usersFile
	} else {
		cfg.UsersFile = "./users.csv"
	}

	return cfg
}
