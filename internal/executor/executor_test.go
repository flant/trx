package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareExecOpts(t *testing.T) {
	tests := []struct {
		name         string
		wd           string
		commands     []string
		env          map[string]string
		templateVars map[string]string
		wantErr      bool
		wantCmd      string
		wantEnv      []string
	}{
		{
			name:         "basic command with env",
			wd:           "/app",
			commands:     []string{"echo Hello, World!"},
			env:          map[string]string{"env": "test"},
			templateVars: map[string]string{},
			wantErr:      false,
			wantCmd:      "set -e\necho Hello, World!",
			wantEnv:      []string{"ENV=test"},
		},
		{
			name:     "command with template vars",
			wd:       "/app",
			commands: []string{"echo {{ .RepoTag }}"},
			env:      map[string]string{"env": "{{ .RepoUrl }}"},
			templateVars: map[string]string{
				"RepoTag":    "v1.0.0",
				"RepoUrl":    "https://example.com/repo.git",
				"RepoCommit": "abcdefg",
			},
			wantErr: false,
			wantCmd: "set -e\necho v1.0.0",
			wantEnv: []string{"ENV=https://example.com/repo.git"},
		},
		{
			name:         "unresolved template",
			wd:           "/app",
			commands:     []string{"echo {{ .UnknownVar }}"},
			env:          map[string]string{},
			templateVars: map[string]string{},
			wantErr:      false,
			wantCmd:      "set -e\necho ",
			wantEnv:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prepareExecOpts(tt.wd, tt.commands, tt.env, tt.templateVars)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCmd, got.cmd)
				assert.Equal(t, tt.wantEnv, got.env)
				assert.Equal(t, tt.wd, got.wd)
			}
		})
	}
}
