package config

type Config struct {
	RPCURLs           []string
	KafkaBrokers      []string
	KafkaTopic        string
	KafkaIdempotent   bool
	KafkaCompression  string
	KafkaRequiredAcks string
	Confirmations     int
	ReorgDepth        int
}

func Default() Config {
	return Config{
		KafkaTopic:    "tx_events",
		Confirmations: 3,
		ReorgDepth:    12,
	}
}
