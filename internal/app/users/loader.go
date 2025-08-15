package users

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

func LoadUsers(file string) map[string]string {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	users := make(map[string]string, 512_000)

	for i := 0; i < 5; i++ {
		records, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatal(err)
		}
		users[records[0]] = records[1]
	}

	fmt.Println("loaded", len(users), "users")

	return users
}
