// Package kubectl provides output formatting utilities for kubectl-forge
package kubectl

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	// OutputFormatTable outputs data as a formatted table
	OutputFormatTable OutputFormat = "table"
	// OutputFormatJSON outputs data as JSON
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatYAML outputs data as YAML
	OutputFormatYAML OutputFormat = "yaml"
)

// ParseOutputFormat validates and returns the output format
func ParseOutputFormat(format string) (OutputFormat, error) {
	switch format {
	case "", "table":
		return OutputFormatTable, nil
	case "json":
		return OutputFormatJSON, nil
	case "yaml":
		return OutputFormatYAML, nil
	default:
		return "", fmt.Errorf("invalid output format: %s (valid: table, json, yaml)", format)
	}
}

// Printer handles output formatting for CLI commands
type Printer struct {
	Format OutputFormat
	Out    io.Writer
}

// NewPrinter creates a new Printer with the specified format
func NewPrinter(format OutputFormat, out io.Writer) *Printer {
	return &Printer{
		Format: format,
		Out:    out,
	}
}

// PrintTable prints data as a table using tabwriter
func (p *Printer) PrintTable(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(p.Out, 0, 0, 3, ' ', 0)

	// Print headers
	for i, h := range headers {
		if i > 0 {
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprint(w, "\t")
		}
		//nolint:errcheck // Writing to output in CLI context
		fmt.Fprint(w, h)
	}
	//nolint:errcheck // Writing to output in CLI context
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				//nolint:errcheck // Writing to output in CLI context
				fmt.Fprint(w, "\t")
			}
			//nolint:errcheck // Writing to output in CLI context
			fmt.Fprint(w, cell)
		}
		//nolint:errcheck // Writing to output in CLI context
		fmt.Fprintln(w)
	}

	return w.Flush()
}

// PrintJSON prints data as indented JSON
func (p *Printer) PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(p.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintYAML prints data as YAML
func (p *Printer) PrintYAML(data interface{}) error {
	encoder := yaml.NewEncoder(p.Out)
	defer func() {
		//nolint:errcheck,gosec // Best-effort close in defer
		encoder.Close()
	}()
	encoder.SetIndent(2)
	return encoder.Encode(data)
}
