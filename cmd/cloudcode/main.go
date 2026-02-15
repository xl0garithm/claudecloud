package main

import (
	"log"
	"os"

	"github.com/logan/cloudcode/internal/config"
)

func main() {
	cfg := config.Load()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("cloudcode starting with provider=%s listen=%s", cfg.Provider, cfg.ListenAddr)
}
