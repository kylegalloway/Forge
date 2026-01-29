package validation

import (
	"testing"

	"github.com/kylegalloway/forge/pkg/apis/common"
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

func boolPtr(b bool) *bool { return &b }

func TestValidateExtraMounts(t *testing.T) {
	tests := []struct {
		name    string
		mounts  []common.ExtraMount
		wantErr bool
	}{
		{
			name: "valid single configMapRef mount",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					MountPath:    "/etc/my-config",
					ReadOnly:     boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "valid single secretRef mount",
			mounts: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "my-secret"},
					MountPath: "/etc/my-secret",
					ReadOnly:  boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "valid mount with subPath",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					MountPath:    "/etc/my-config/app.conf",
					SubPath:      "app.conf",
					ReadOnly:     boolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid neither ref set",
			mounts: []common.ExtraMount{
				{
					MountPath: "/etc/my-config",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid both refs set",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					SecretRef:    &common.LocalObjectReference{Name: "my-secret"},
					MountPath:    "/etc/my-mount",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid relative mountPath",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					MountPath:    "relative/path",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid reserved mountPath /workspace",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "my-config"},
					MountPath:    "/workspace",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid reserved mountPath /tmp",
			mounts: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "my-secret"},
					MountPath: "/tmp",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid reserved mountPath /var/run/secrets/kubernetes.io/serviceaccount",
			mounts: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "my-secret"},
					MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid duplicate mountPath",
			mounts: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "config-a"},
					MountPath:    "/etc/my-config",
				},
				{
					SecretRef: &common.LocalObjectReference{Name: "secret-b"},
					MountPath: "/etc/my-config",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtraMounts(tt.mounts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExtraMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeExtraMounts(t *testing.T) {
	tests := []struct {
		name      string
		topLevel  []common.ExtraMount
		perAction []common.ExtraMount
		wantLen   int
		wantErr   bool
	}{
		{
			name: "top-level only",
			topLevel: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "global-config"},
					MountPath:    "/etc/global-config",
				},
			},
			perAction: nil,
			wantLen:   1,
			wantErr:   false,
		},
		{
			name:     "per-action only",
			topLevel: nil,
			perAction: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "action-secret"},
					MountPath: "/etc/action-secret",
				},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "both combined successfully",
			topLevel: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "global-config"},
					MountPath:    "/etc/global-config",
				},
			},
			perAction: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "action-secret"},
					MountPath: "/etc/action-secret",
				},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "conflict between top-level and per-action duplicate mountPath",
			topLevel: []common.ExtraMount{
				{
					ConfigMapRef: &common.LocalObjectReference{Name: "global-config"},
					MountPath:    "/etc/shared-path",
				},
			},
			perAction: []common.ExtraMount{
				{
					SecretRef: &common.LocalObjectReference{Name: "action-secret"},
					MountPath: "/etc/shared-path",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeExtraMounts(tt.topLevel, tt.perAction)
			if (err != nil) != tt.wantErr {
				t.Errorf("MergeExtraMounts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("MergeExtraMounts() returned %d mounts, want %d", len(got), tt.wantLen)
			}
		})
	}
}
