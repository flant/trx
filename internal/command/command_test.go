package command

import (
	"bytes"
	"context"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A single stdout line larger than the 64 KiB bufio.Scanner token limit used to
// stop the reading goroutine, block the child on a full pipe and hang the run
// (while holding the execution lock) because cmd.Wait was never reached.
func TestExecute_longLineDoesNotDeadlock(t *testing.T) {
	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	err := execute(ctx, &excuteOpts{
		cmd: `head -c 200000 /dev/zero | tr '\0' 'a'; echo; echo done >&2`,
	})
	require.NoError(t, err)
	require.Equal(t, 200000, strings.Count(out.String(), "a"))
	require.Contains(t, out.String(), "done")
}

// A process left running in the background inherits the output and keeps it
// open after the command itself is gone, which used to hold the execution lock
// forever. The wait is bounded, and the command still counts as succeeded.
func TestExecute_backgroundProcessDoesNotHang(t *testing.T) {
	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	outputWaitDelay = 100 * time.Millisecond
	t.Cleanup(func() { outputWaitDelay = 10 * time.Second })

	done := make(chan error, 1)
	go func() { done <- execute(context.Background(), &excuteOpts{cmd: "sleep 60 & echo started"}) }()

	select {
	case err := <-done:
		require.NoError(t, err)
		require.Contains(t, out.String(), "started")
	case <-time.After(20 * time.Second):
		t.Fatal("execute did not return while a background process held the output")
	}
}

func TestExecute_failureReportsStderr(t *testing.T) {
	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	err := execute(context.Background(), &excuteOpts{cmd: "echo boom >&2; exit 3"})
	require.ErrorContains(t, err, "exit status 3")
	require.Contains(t, out.String(), "boom")
}

// Commands were rendered with html/template, so a tag such as v1.0.0+build.3
// reached sh as v1.0.0&#43;build.3.
func TestResolveTemplate_doesNotEscape(t *testing.T) {
	vars := map[string]string{
		"RepoTag": "v1.0.0+build.3",
		"RepoUrl": "https://example.com/a.git?x=1&y=2",
	}

	got, err := resolveTemplate(`deploy {{ .RepoTag }} from "{{ .RepoUrl }}"`, vars)
	require.NoError(t, err)
	require.Equal(t, `deploy v1.0.0+build.3 from "https://example.com/a.git?x=1&y=2"`, got)
}

// Viper lower-cases config keys, and the repository-config path used to replace
// the environment wholesale without upper-casing them again, so commands saw
// foo=bar and tools such as werf or kubectl ignored it.
func TestSetEnv_upperCasesNames(t *testing.T) {
	e, err := NewExecutor(context.Background(), map[string]string{"werf_env": "production"}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"WERF_ENV=production"}, e.Env)

	e.SetEnv(map[string]string{"werf_env": "staging", "kubeconfig": "/tmp/kc"})
	require.Equal(t, []string{"KUBECONFIG=/tmp/kc", "WERF_ENV=staging"}, e.Env)
}

// exec.CommandContext killed sh alone, so the actual workload survived the
// signal as an orphan outside the execution lock.
func TestExecute_cancelStopsTheWholeProcessGroup(t *testing.T) {
	log.SetOutput(io.Discard)

	dir := t.TempDir()
	marker := filepath.Join(dir, "orphan-survived")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- execute(ctx, &excuteOpts{
			cmd: "(sleep 2; touch " + marker + ") & echo started; sleep 30",
		})
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("execute did not return after the context was canceled")
	}

	time.Sleep(3 * time.Second)
	require.NoFileExists(t, marker, "the background workload outlived the canceled run")
}

// A hook reports what happened to the run, so canceling the run must not stop
// it from running.
func TestExecHook_runsAfterCancellation(t *testing.T) {
	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	ctx, cancel := context.WithCancel(context.Background())
	e, err := NewExecutor(ctx, nil, nil)
	require.NoError(t, err)
	cancel()

	require.Error(t, e.Exec([]string{"echo command"}))
	require.NoError(t, e.ExecHook([]string{"echo hook reported the failure"}))
	require.Contains(t, out.String(), "hook reported the failure")
}

// A workload that traps SIGTERM must not outlive the canceled run either:
// os/exec only kills sh itself when the wait delay expires.
func TestExecute_cancelKillsAProcessIgnoringSigterm(t *testing.T) {
	log.SetOutput(io.Discard)

	outputWaitDelay = 500 * time.Millisecond
	t.Cleanup(func() { outputWaitDelay = 10 * time.Second })

	dir := t.TempDir()
	marker := filepath.Join(dir, "survived")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- execute(ctx, &excuteOpts{
			cmd: "(trap '' TERM; sleep 3; touch " + marker + ") & sleep 30",
		})
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("execute did not return after the context was canceled")
	}

	time.Sleep(4 * time.Second)
	require.NoFileExists(t, marker, "a SIGTERM-ignoring workload outlived the canceled run")
}

// A misspelled variable used to be rendered as the literal <no value> and
// handed to sh as an argument.
func TestResolveTemplate_unknownVariableIsAnError(t *testing.T) {
	_, err := resolveTemplate(`deploy {{ .RepoTagg }}`, map[string]string{"RepoTag": "v1.0.0"})
	require.ErrorContains(t, err, "RepoTagg")
}

// A variable that is only filled in for one hook must still render for the
// others, or a shared env value mentioning it would break every one of them.
func TestResolveTemplate_knownButUnsetVariableRendersEmpty(t *testing.T) {
	got, err := resolveTemplate(`quorum "{{ .FailedQuorumName }}" failed`, map[string]string{
		"RepoTag":          "v1.0.0",
		"FailedQuorumName": "",
	})
	require.NoError(t, err)
	require.Equal(t, `quorum "" failed`, got)
}
