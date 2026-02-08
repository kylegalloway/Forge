// Package main provides color output support for kubectl-forge
package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// ColorWriter wraps an io.Writer and adds color support
type ColorWriter struct {
	out      io.Writer
	useColor bool
}

// NewColorWriter creates a new ColorWriter
// Color is enabled by default if output is a terminal
func NewColorWriter(out io.Writer) *ColorWriter {
	useColor := false
	if f, ok := out.(*os.File); ok {
		useColor = term.IsTerminal(int(f.Fd()))
	}
	return &ColorWriter{out: out, useColor: useColor}
}

// SetColor enables or disables color output
func (c *ColorWriter) SetColor(enabled bool) {
	c.useColor = enabled
}

// Write implements io.Writer
func (c *ColorWriter) Write(p []byte) (n int, err error) {
	return c.out.Write(p)
}

// Success writes text in green
func (c *ColorWriter) Success(format string, a ...interface{}) {
	c.writeColored(colorGreen, format, a...)
}

// Error writes text in red
func (c *ColorWriter) Error(format string, a ...interface{}) {
	c.writeColored(colorRed, format, a...)
}

// Warning writes text in yellow
func (c *ColorWriter) Warning(format string, a ...interface{}) {
	c.writeColored(colorYellow, format, a...)
}

// Info writes text in cyan
func (c *ColorWriter) Info(format string, a ...interface{}) {
	c.writeColored(colorCyan, format, a...)
}

// Bold writes text in bold
func (c *ColorWriter) Bold(format string, a ...interface{}) {
	c.writeColored(colorBold, format, a...)
}

// Phase returns a colorized phase string
func (c *ColorWriter) Phase(phase string) string {
	if !c.useColor {
		return phase
	}
	switch phase {
	case "Completed", "Succeeded":
		return colorGreen + phase + colorReset
	case "Failed":
		return colorRed + phase + colorReset
	case "Running":
		return colorBlue + phase + colorReset
	case "Pending":
		return colorYellow + phase + colorReset
	case "Retrying":
		return colorPurple + phase + colorReset
	case "Queued":
		return colorCyan + phase + colorReset
	default:
		return phase
	}
}

// Status returns a colorized status marker
func (c *ColorWriter) Status(success bool) string {
	if !c.useColor {
		if success {
			return "+"
		}
		return "X"
	}
	if success {
		return colorGreen + "✓" + colorReset
	}
	return colorRed + "✗" + colorReset
}

// WarningMarker returns a warning marker
func (c *ColorWriter) WarningMarker() string {
	if !c.useColor {
		return "!"
	}
	return colorYellow + "!" + colorReset
}

func (c *ColorWriter) writeColored(color, format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	if c.useColor {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprintf(c.out, "%s%s%s", color, text, colorReset)
	} else {
		//nolint:errcheck // Writing to stdout in CLI context
		fmt.Fprint(c.out, text)
	}
}
