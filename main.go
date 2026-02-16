package main

import (
	"fmt"
	"log"

	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	// Set the current user and update the file
	err = cfg.SetUser("philip")
	if err != nil {
		log.Fatalf("error setting user: %v", err)
	}

	newCfg, err := config.Read()
	if err != nil {
		log.Fatalf("error reading updated config: %v", err)
	}
	fmt.Printf("Config: %+v\n", newCfg)
}
