package main

import (
	"fmt"
	"os"

	"github.com/philip-hargreaves/feed-ingestion-service/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		fmt.Printf("error reading config: %v\n", err)
		os.Exit(1)
	}

	err = cfg.SetUser("philip")
	if err != nil {
		fmt.Printf("error setting user: %v\n", err)
		os.Exit(1)
	}

	cfg, err = config.Read()
	if err != nil {
		fmt.Printf("error reading config again: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config: %+v\n", cfg)
}
