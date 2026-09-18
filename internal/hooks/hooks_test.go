package hooks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"trx/internal/config"
	"trx/internal/templates"
)

func TestRunHooks(t *testing.T) {
	repoTemplatevars := templates.GetRepoTemplateVars(templates.RepoTemplateVarsData{
		RepoTag:    "v1.0.0",
		RepoUrl:    "https://github.com/stretchr/testify/assert",
		RepoCommit: "1234567890abcdef",
	})
	hookExecutor, err := NewHookMockExecutor(context.Background(), &config.Config{
		Hooks: config.Hooks{
			Env:              map[string]string{"ENV": "test", "MSG": "task {{ .FailedTaskName }} failed"},
			OnCommandStarted: &[]string{"commit: {{ .RepoCommit }}, tag: {{ .RepoTag }}, url: {{ .RepoUrl }}"},
			OnCommandFailure: &[]string{`T_FAILED -- {{ .FailedTaskName }}`},
		},
	}, HookExecutorOptions{
		TemplateVars: repoTemplatevars,
	})
	assert.NoError(t, err)

	err = hookExecutor.RunOnCommandStartedHook("name")
	assert.NoError(t, err)
}
