package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
)

func initLogger(cmd *cobra.Command, args []string) error {
	level := new(slog.Level)
	err := level.UnmarshalText([]byte(logLevel))
	if err != nil {
		return fmt.Errorf("invalid error level: %w", err)
	}

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(h))
	return nil
}
