package utils

import (
	"bufio"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
)

func scanAll(t *testing.T, r io.Reader) []string {
	t.Helper()
	scanner := NewOutputScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	assert.NoError(t, scanner.Err())
	return lines
}

func TestScanLinesCollapseCR(t *testing.T) {
	cases := map[string][]string{
		"a\nb\r\nc":                 {"a", "b", "c"},
		"a\rb\r\n\n":                {"b", ""},
		"spin /\rspin -\rdone\n":    {"done"},
		"ends with a carriage\r":    {"ends with a carriage"},
		"\r\n\r\n":                  {"", ""},
		"text\r\r\nnext\n":          {"text", "next"},
		"last line without newline": {"last line without newline"},
	}
	for input, want := range cases {
		assert.Equal(t, want, scanAll(t, strings.NewReader(input)), "%q", input)
		// One byte per read, so "\r\n" arrives split across reads.
		assert.Equal(t, want, scanAll(t, iotest.OneByteReader(strings.NewReader(input))), "%q one byte at a time", input)
	}
}

// A spinner that redraws one line with "\r" for a long time used to produce a
// single line over bufio's 64 KB limit, which stopped the scanner. Its frames
// now collapse into the text that follows the last one.
func TestOutputScannerSpinner(t *testing.T) {
	spinner := strings.Repeat("Ensure prerequisites for \"firefox\" are available /\r", 25000)
	input := spinner + "after the spinner\n# [INFO    ] Completed state [firefox]\n"
	assert.Greater(t, len(spinner), MaxOutputLine)

	lines := scanAll(t, strings.NewReader(input))
	assert.Equal(t, []string{"after the spinner", "# [INFO    ] Completed state [firefox]"}, lines)
}

// A line over MaxOutputLine is split into pieces, and the lines after it are
// still scanned.
func TestOutputScannerLongLine(t *testing.T) {
	input := strings.Repeat("x", 2*MaxOutputLine) + "\nafter\n" + strings.Repeat("y", MaxOutputLine+10)
	lines := scanAll(t, strings.NewReader(input))
	assert.Equal(t, []string{
		strings.Repeat("x", MaxOutputLine),
		strings.Repeat("x", MaxOutputLine),
		"",
		"after",
		strings.Repeat("y", MaxOutputLine),
		"yyyyyyyyyy",
	}, lines)
}

// If the scanner stops early, DrainOutput must keep reading so the writer,
// standing in for salt-call, never blocks on a full pipe.
func TestDrainOutputUnblocksWriter(t *testing.T) {
	r, w := io.Pipe()
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	written := make(chan error, 1)
	go func() {
		_, err := io.WriteString(w, strings.Repeat("x", 2*MaxOutputLine)+"\nmore output after the long line\n")
		w.Close()
		written <- err
	}()

	read := make(chan error, 1)
	go func() {
		// Read one line, then stop the way a scanner error would.
		scanner := bufio.NewScanner(r)
		scanner.Scan()
		read <- DrainOutput(r)
	}()

	timeout := time.After(10 * time.Second)
	select {
	case err := <-read:
		assert.NoError(t, err)
	case <-timeout:
		t.Fatal("reading the output hung")
	}
	select {
	case err := <-written:
		assert.NoError(t, err)
	case <-timeout:
		t.Fatal("the writer blocked after the scanner stopped")
	}
}
