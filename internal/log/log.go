package log

import (
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
	cblog "github.com/charmbracelet/log"
)

// Logger embeds the Charm Logger and adds Printf/Fatalf
type Logger struct{ *cblog.Logger }

// Printf routes goose/info-style logs through Infof.
func (l *Logger) Printf(format string, v ...any) { l.Infof(format, v...) }

// Fatalf keeps goose's contract of exiting the program.
func (l *Logger) Fatalf(format string, v ...any) { l.Logger.Fatalf(format, v...) }

var (
	logger     *Logger
	initLogger sync.Once
)

// GetLogger returns a logger instance
func GetLogger() *Logger {
	initLogger.Do(func() {
		styles := cblog.DefaultStyles()
		styles.Levels[cblog.FatalLevel] = lipgloss.NewStyle().SetString("FATAL")
		styles.Levels[cblog.ErrorLevel] = lipgloss.NewStyle().SetString("ERROR")
		styles.Levels[cblog.WarnLevel] = lipgloss.NewStyle().SetString("WARN")
		styles.Levels[cblog.InfoLevel] = lipgloss.NewStyle().SetString("INFO")

		base := cblog.New(os.Stderr)
		base.SetStyles(styles)
		base.SetReportTimestamp(false)

		logger = &Logger{base}
	})
	return logger
}

// * Convenience wrappers

func Info(msg any, keyvals ...any)   { GetLogger().Logger.Info(msg, keyvals...) }
func Infof(format string, v ...any)  { GetLogger().Logger.Infof(format, v...) }
func Warn(msg any, keyvals ...any)   { GetLogger().Logger.Warn(msg, keyvals...) }
func Warnf(format string, v ...any)  { GetLogger().Logger.Warnf(format, v...) }
func Error(msg any, keyvals ...any)  { GetLogger().Logger.Error(msg, keyvals...) }
func Errorf(format string, v ...any) { GetLogger().Logger.Errorf(format, v...) }
func Fatal(msg any, keyvals ...any)  { GetLogger().Logger.Fatal(msg, keyvals...) }
func Fatalf(format string, v ...any) { GetLogger().Logger.Fatalf(format, v...) }
