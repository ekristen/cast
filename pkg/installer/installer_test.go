package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

// A fake salt-call that writes a progress spinner redrawn with "\r" for 2 MB,
// the shape `snap install` output takes when a slow install wraps it, followed
// by one completed state. Before the fix, cast's stderr scanner stopped at the
// 64 KB line, salt-call blocked on the full pipe, and runSaltstack never returned.
const fakeSaltCall = `#!/bin/sh
i=0
while [ $i -lt 40000 ]; do
  printf 'Ensure prerequisites for "firefox" are available /\r' >&2
  i=$((i+1))
done
echo >&2
echo '# [INFO    ] Running state [firefox] at time 07:08:02.000000' >&2
echo '# [INFO    ] Completed state [firefox] at time 08:41:55.000000 (duration_in_ms=5633528.227)' >&2
echo 'local: {}'
`

func TestRunSaltstackSurvivesLongOutputLine(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "salt-call")
	assert.NoError(t, os.WriteFile(command, []byte(fakeSaltCall), 0755))

	// Cancelling the context kills a hung fake salt-call when the test ends.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hook := logtest.NewGlobal()
	t.Cleanup(hook.Reset)

	i := &Installer{
		ctx:     ctx,
		log:     logrus.WithField("component", "installer"),
		config:  &Config{SaltStackLogLevel: "info", SaltStackState: "test"},
		logFile: filepath.Join(dir, "saltstack.log"),
		outFile: filepath.Join(dir, "results.yaml"),
		command: command,
	}

	done := make(chan error, 1)
	go func() { done <- i.runSaltstack() }()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(60 * time.Second):
		t.Fatal("runSaltstack hung on a long salt-call output line")
	}

	log, err := os.ReadFile(i.logFile)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(log), "Completed state [firefox]"), "the log is missing output after the long line")

	// Cast must still parse the state that follows the long line.
	parsed := false
	for _, e := range hook.AllEntries() {
		if e.Message == "state completed" && e.Data["state"] == "firefox" {
			parsed = true
		}
	}
	assert.True(t, parsed, "cast did not parse the state after the long line")
}
