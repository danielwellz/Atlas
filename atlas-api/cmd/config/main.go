package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/atlas/atlas-api/internal/config"
)

func main() {
	if len(os.Args) != 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	command := os.Args[1]
	if command != "print" && command != "validate" {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "validate":
		fmt.Printf("configuration valid for APP_ENV=%s\n", cfg.Env)
	case "print":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(cfg.Redacted()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to print config: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage(w *os.File) {
	_, _ = fmt.Fprintln(w, "Usage: go run ./cmd/config [print|validate]")
}
