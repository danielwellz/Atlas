package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/atlas/atlas-api/internal/config"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/exercise"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "exercises":
		err = runExercises(os.Args[2:])
	default:
		printUsage(os.Stderr)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "seed command failed: %v\n", err)
		os.Exit(1)
	}
}

func runExercises(args []string) error {
	flagSet := flag.NewFlagSet("exercises", flag.ContinueOnError)
	filePath := flagSet.String("file", "./seed/exercises.csv", "path to exercises CSV file")
	if err := flagSet.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := zap.NewNop()
	if cfg.Env == config.EnvLocal {
		logger = zap.Must(zap.NewDevelopment())
	}
	defer func() {
		_ = logger.Sync()
	}()

	database, err := sql.Open("pgx", cfg.PostgresURL)
	if err != nil {
		return fmt.Errorf("open postgres connection: %w", err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := db.New(database).WithTx(tx)
	result, err := exercise.ImportExercisesCSV(ctx, queries, *filePath)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	logger.Info("exercise seed import complete",
		zap.Int("rows_processed", result.RowsProcessed),
		zap.String("file", *filePath),
	)
	return nil
}

func printUsage(w *os.File) {
	_, _ = fmt.Fprintln(w, "Usage: go run ./cmd/seed exercises --file ./seed/exercises.csv")
}
