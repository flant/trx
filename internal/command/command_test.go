package command

import (
	"bytes"
	"context"
	"io"
	"log"
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
