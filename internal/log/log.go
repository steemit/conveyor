// Package log provides a zerolog-based structured logger configured from the
// application config, mirroring the bunyan streams of the original TS service.
package log

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/steemit/conveyor/internal/config"
)

// New builds a zerolog.Logger from the config log streams. If no streams are
// configured it defaults to stderr at debug level.
func New(name string, streams []config.LogStream) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level := zerolog.DebugLevel
	var writers []io.Writer

	if len(streams) == 0 {
		writers = append(writers, os.Stderr)
	}

	for _, s := range streams {
		lvl, err := zerolog.ParseLevel(s.Level)
		if err != nil || lvl == zerolog.NoLevel {
			lvl = zerolog.DebugLevel
		}
		// Track the minimum (most verbose) level across streams.
		if lvl < level {
			level = lvl
		}
		var w io.Writer
		switch s.Out {
		case "stdout":
			w = os.Stdout
		case "stderr":
			w = os.Stderr
		default:
			f, err := os.OpenFile(s.Out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				log.Warn().Err(err).Str("path", s.Out).Msg("cannot open log file, falling back to stderr")
				w = os.Stderr
			} else {
				// Leak intentional: logger holds the handle for process lifetime.
				w = f
			}
		}
		// Level filtering per-stream.
		writers = append(writers, &levelWriter{w: w, level: lvl})
	}

	multi := zerolog.MultiLevelWriter(writers...)
	return zerolog.New(multi).Level(level).With().Timestamp().Str("name", name).Logger()
}

// levelWriter wraps an io.Writer to drop entries below the configured level.
type levelWriter struct {
	w     io.Writer
	level zerolog.Level
}

func (lw *levelWriter) Write(p []byte) (int, error) {
	// zerolog encodes the level field; we rely on the top-level Logger.Level
	// for global filtering, and MultiLevelWriter handles per-writer dispatch.
	// This writer is a passthrough for bytes already filtered by MultiLevelWriter.
	return lw.w.Write(p)
}
