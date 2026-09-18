package executor

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single stdout line larger than bufio.Scanner's 64 KiB token limit used to
// deadlock the run while holding the execution lock.
func TestExecuteLongOutputLine(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	done := make(chan error, 1)
	go func() {
		done <- execute(context.Background(), &excuteOpts{
			cmd: "head -c 200000 /dev/zero | tr '\\0' 'x'",
			wd:  t.TempDir(),
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("execute deadlocked on a long output line")
	}
}

func TestExecuteFailureKeepsStderr(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	err := execute(context.Background(), &excuteOpts{
		cmd: "echo boom-on-stderr >&2\nexit 3",
		wd:  t.TempDir(),
	})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "boom-on-stderr"), err.Error())
}
