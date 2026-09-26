// Package logging builds the loggers the binaries use.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// New returns a logger that writes to both stderr and path. The caller closes the returned file.
func New(path string) (*slog.Logger, *os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, file), nil)), file, nil
}
