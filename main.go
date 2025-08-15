package main

import (
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

func main() {
	f, err := os.Create("users.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	csv := csv.NewWriter(f)
	defer csv.Flush()

	uid := 1
	emit := func(addr string) {
		if err := csv.Write([]string{common.HexToAddress(addr).Hex(), fmt.Sprintf("u%06d", uid)}); err != nil {
			panic(err)
		}
		uid++
	}

	realAddrs := []string{
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7",
	}

	for _, addr := range realAddrs {
		emit(addr)
	}

	for uid <= 500_000 {
		var b [20]byte
		if _, err := rand.Read(b[:]); err != nil {
			log.Fatal(err)
		}
		addr := common.BytesToAddress(b[:]).Hex()
		emit(addr)
	}

	log.Println("done")
}
