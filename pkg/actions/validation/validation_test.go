package validation

import (
	"testing"
)

func TestValidateExtraArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    []string{"--max-package-size", "100", "--no-progress"},
			wantErr: false,
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "nil args",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "args with spaces",
			args:    []string{"--message", "hello world"},
			wantErr: false,
		},
		{
			name:    "semicolon injection",
			args:    []string{"--flag; rm -rf /"},
			wantErr: true,
		},
		{
			name:    "pipe injection",
			args:    []string{"--flag | cat /etc/passwd"},
			wantErr: true,
		},
		{
			name:    "ampersand injection",
			args:    []string{"--flag && malicious"},
			wantErr: true,
		},
		{
			name:    "dollar sign injection",
			args:    []string{"--flag $HOME"},
			wantErr: true,
		},
		{
			name:    "backtick injection",
			args:    []string{"--flag `whoami`"},
			wantErr: true,
		},
		{
			name:    "parentheses injection",
			args:    []string{"--flag $(whoami)"},
			wantErr: true,
		},
		{
			name:    "brace injection",
			args:    []string{"--flag ${PATH}"},
			wantErr: true,
		},
		{
			name:    "redirect injection",
			args:    []string{"--flag > /tmp/out"},
			wantErr: true,
		},
		{
			name:    "newline injection",
			args:    []string{"--flag\nrm -rf /"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtraArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtraArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "string with spaces",
			input: "hello world",
			want:  "'hello world'",
		},
		{
			name:  "string with single quote",
			input: "it's",
			want:  "'it'\"'\"'s'",
		},
		{
			name:  "string with double quote",
			input: `say "hello"`,
			want:  `'say "hello"'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShellEscape(tt.input); got != tt.want {
				t.Errorf("ShellEscape() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendExtraArgs(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "append simple args",
			cmd:  "zarf package create .",
			args: []string{"--no-progress", "--skip-sbom"},
			want: "zarf package create . --no-progress --skip-sbom",
		},
		{
			name: "append args with values",
			cmd:  "zarf package create .",
			args: []string{"--max-package-size", "100"},
			want: "zarf package create . --max-package-size 100",
		},
		{
			name:    "reject dangerous args",
			cmd:     "zarf package create .",
			args:    []string{"--flag; rm -rf /"},
			wantErr: true,
		},
		{
			name: "escape args with spaces",
			cmd:  "zarf package create .",
			args: []string{"--message", "hello world"},
			want: "zarf package create . --message 'hello world'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppendExtraArgs(tt.cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppendExtraArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("AppendExtraArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
