package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"
)

type LogHandler struct {
	w     io.Writer
	level slog.Leveler
}

func (l *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return l
}

func (l *LogHandler) WithGroup(name string) slog.Handler {
	return l
}

func NewLogHandler(w io.Writer, level slog.Leveler) *LogHandler {
	return &LogHandler{w: w, level: level}
}

func (l *LogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= l.level.Level()
}

func (l *LogHandler) Handle(_ context.Context, record slog.Record) error {
	format := fmt.Sprintf("[%s] %s", record.Time.Format(time.DateTime), record.Level)
	if source := record.Source(); source != nil {
		format += fmt.Sprintf(" %s:%d", source.File, source.Line)
	}
	format += fmt.Sprintf(" %s", record.Message)
	record.Attrs(func(attr slog.Attr) bool {
		format += fmt.Sprintf(attr.Key+" ", attr.Value.String())
		return true
	})
	_, err := io.WriteString(l.w, format+"\n")
	return err
}
