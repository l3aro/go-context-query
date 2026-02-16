package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents log severity levels
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger interface defines structured logging methods
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	SetLevel(level Level)
	SetJSONOutput(enabled bool)
}

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Level      Level
	JSONOutput bool
	Stdout     io.Writer
	Stderr     io.Writer
}

// DefaultLogger is the default implementation of Logger
type DefaultLogger struct {
	mu         sync.Mutex
	level      Level
	jsonOutput bool
	stdout     io.Writer
	stderr     io.Writer
	colors     bool
}

var (
	defaultLogger *DefaultLogger
	once          sync.Once
)

// New creates a new logger with the given configuration
func New(cfg LoggerConfig) *DefaultLogger {
	l := &DefaultLogger{
		level:      cfg.Level,
		jsonOutput: cfg.JSONOutput,
		stdout:     cfg.Stdout,
		stderr:     cfg.Stderr,
		colors:     isTerminal(cfg.Stderr),
	}

	// Default to os.Stdout/os.Stderr if not provided
	if l.stdout == nil {
		l.stdout = os.Stdout
	}
	if l.stderr == nil {
		l.stderr = os.Stderr
	}

	return l
}

// Default returns the default logger instance
func Default() *DefaultLogger {
	once.Do(func() {
		defaultLogger = New(LoggerConfig{
			Level:  InfoLevel,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	})
	return defaultLogger
}

// isTerminal checks if the writer is a terminal
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isTerminalFD(f)
	}
	return false
}

// isTerminalFD checks if the file descriptor is a terminal
func isTerminalFD(f *os.File) bool {
	return isTerminalPath(f.Name())
}

// isTerminalPath checks if the path is a terminal device
func isTerminalPath(path string) bool {
	// Check if stdout/stderr is redirected
	return isatty()
}

// isatty checks if stdout is connected to a terminal
func isatty() bool {
	return isTTY()
}

// IsTTY checks if the standard output is a TTY
func IsTTY() bool {
	return isTTY()
}

// isTTY returns true if stdout is a terminal
func isTTY() bool {
	return runtime.GOOS != "windows" && IsTerminalWidth() > 0
}

// IsTerminalWidth returns the terminal width (0 if not a terminal)
func IsTerminalWidth() int {
	// Simple check - in a real implementation, use termios
	// For now, check if NO_COLOR is set
	if os.Getenv("NO_COLOR") != "" {
		return 0
	}
	return 80 // Default assumption
}

// formatMessage formats the message with key-value args
func formatMessage(msg string, args ...interface{}) string {
	if len(args) == 0 {
		return msg
	}

	var sb strings.Builder
	sb.WriteString(msg)

	if len(args)%2 != 0 {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%v", args[0]))
		args = args[1:]
	}

	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		sb.WriteString(" ")
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprintf("%v", args[i+1]))
	}

	return sb.String()
}

// colorize wraps the message with ANSI color codes if colors are enabled
func (l *DefaultLogger) colorize(level Level, msg string) string {
	if !l.colors {
		return msg
	}

	color := getColor(level)
	reset := "\033[0m"
	return color + msg + reset
}

// getColor returns the ANSI color code for the given level
func getColor(level Level) string {
	switch level {
	case DebugLevel:
		return "\033[36m" // Cyan
	case InfoLevel:
		return "\033[32m" // Green
	case WarnLevel:
		return "\033[33m" // Yellow
	case ErrorLevel:
		return "\033[31m" // Red
	default:
		return ""
	}
}

// write outputs the log message
func (l *DefaultLogger) write(level Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()

	if l.jsonOutput {
		entry := map[string]interface{}{
			"timestamp": timestamp,
			"level":     levelStr,
			"message":   msg,
		}
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.stderr, string(data))
		return
	}

	// Formatted output with colors
	coloredMsg := l.colorize(level, msg)
	fmt.Fprintf(l.stderr, "[%s] %s: %s\n", timestamp, levelStr, coloredMsg)
}

// Debug logs a debug message
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if l.level > DebugLevel {
		return
	}
	l.write(DebugLevel, formatMessage(msg, args...))
}

// Info logs an info message
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	if l.level > InfoLevel {
		return
	}
	l.write(InfoLevel, formatMessage(msg, args...))
}

// Warn logs a warning message
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	if l.level > WarnLevel {
		return
	}
	l.write(WarnLevel, formatMessage(msg, args...))
}

// Error logs an error message
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	if l.level > ErrorLevel {
		return
	}
	l.write(ErrorLevel, formatMessage(msg, args...))
}

// SetLevel sets the minimum log level
func (l *DefaultLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetJSONOutput enables or disables JSON output
func (l *DefaultLogger) SetJSONOutput(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonOutput = enabled
}

// ProgressSpinner provides a spinner for long-running operations
type ProgressSpinner struct {
	mu       sync.Mutex
	message  string
	spinner  []string
	current  int
	active   bool
	writer   io.Writer
	colors   bool
	stopChan chan struct{}
}

// NewProgressSpinner creates a new progress spinner
func NewProgressSpinner(message string) *ProgressSpinner {
	return &ProgressSpinner{
		message:  message,
		spinner:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current:  0,
		active:   false,
		writer:   os.Stderr,
		colors:   isTTY(),
		stopChan: make(chan struct{}),
	}
}

// Start begins the spinner animation
func (p *ProgressSpinner) Start() {
	p.mu.Lock()
	p.active = true
	p.mu.Unlock()

	go p.animate()
}

// Stop stops the spinner
func (p *ProgressSpinner) Stop() {
	p.mu.Lock()
	p.active = false
	p.mu.Unlock()

	// Wait for animation to stop
	time.Sleep(50 * time.Millisecond)

	// Clear the spinner line
	fmt.Fprint(p.writer, "\r\033[K")
}

// Message updates the spinner message
func (p *ProgressSpinner) Message(msg string) {
	p.mu.Lock()
	p.message = msg
	p.mu.Unlock()
}

// animate runs the spinner animation
func (p *ProgressSpinner) animate() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			if !p.active {
				p.mu.Unlock()
				return
			}
			p.draw()
			p.mu.Unlock()
		case <-p.stopChan:
			return
		}
	}
}

// draw renders the spinner to the terminal
func (p *ProgressSpinner) draw() {
	spinnerChar := p.spinner[p.current%len(p.spinner)]
	p.current++

	if p.colors {
		fmt.Fprintf(p.writer, "\r\033[36m%s\033[0m %s", spinnerChar, p.message)
	} else {
		fmt.Fprintf(p.writer, "\r%s %s", spinnerChar, p.message)
	}
}

// ProgressWriter is a wrapper that shows progress for long operations
type ProgressWriter struct {
	writer  io.Writer
	total   int64
	written int64
	spinner *ProgressSpinner
	lastPct int
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(writer io.Writer, total int64, message string) *ProgressWriter {
	pw := &ProgressWriter{
		writer:  writer,
		total:   total,
		spinner: NewProgressSpinner(message),
	}
	pw.spinner.Start()
	return pw
}

// Write implements io.Writer
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.written += int64(n)

	if pw.total > 0 {
		pct := int(float64(pw.written) / float64(pw.total) * 100)
		if pct != pw.lastPct && pct%10 == 0 {
			pw.spinner.Message(fmt.Sprintf("Processing... %d%%", pct))
			pw.lastPct = pct
		}
	}

	return n, err
}

// Close finishes the progress writer
func (pw *ProgressWriter) Close() {
	if pw.spinner != nil {
		pw.spinner.Stop()
	}
}

// LoggerFromContext gets a logger from context (for future context integration)
func LoggerFromContext() Logger {
	return Default()
}
