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

func TestScanLinesOrCR(t *testing.T) {
	cases := map[string][]string{
		"a\nb\r\nc":                 {"a", "b", "c"},
		"a\rb\r\n\n":                {"a", "b", ""},
		"spin /\rspin -\rdone\n":    {"spin /", "spin -", "done"},
		"ends with a carriage\r":    {"ends with a carriage"},
		"\r\n\r\n":                  {"", ""},
		"last line without newline": {"last line without newline"},
	}
	for input, want := range cases {
		assert.Equal(t, want, scanAll(t, strings.NewReader(input)), "%q", input)
		// One byte per read, so "\r\n" arrives split across reads.
		assert.Equal(t, want, scanAll(t, iotest.OneByteReader(strings.NewReader(input))), "%q one byte at a time", input)
	}
}

// A spinner that redraws one line with "\r" for a long time used to produce a
// single line over bufio's 64 KB limit, which stopped the scanner.
func TestOutputScannerSpinner(t *testing.T) {
	spinner := strings.Repeat("Ensure prerequisites for \"firefox\" are available /\r", 25000)
	input := spinner + "after the spinner\n# [INFO    ] Completed state [firefox]\n"
	assert.Greater(t, len(spinner), MaxOutputLine)

	lines := scanAll(t, strings.NewReader(input))
	assert.Equal(t, 25002, len(lines))
	assert.Equal(t, "# [INFO    ] Completed state [firefox]", lines[len(lines)-1])
}

// The scanner stops at a line over MaxOutputLine. DrainOutput must keep reading
// so the writer, standing in for salt-call, never blocks on a full pipe.
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

	read := make(chan error, 2)
	go func() {
		scanner := NewOutputScanner(r)
		for scanner.Scan() {
		}
		read <- scanner.Err()
		read <- DrainOutput(r)
	}()

	timeout := time.After(10 * time.Second)
	for _, check := range []func(error){
		func(err error) { assert.ErrorIs(t, err, bufio.ErrTooLong) },
		func(err error) { assert.NoError(t, err) },
	} {
		select {
		case err := <-read:
			check(err)
		case <-timeout:
			t.Fatal("reading the output hung")
		}
	}
	select {
	case err := <-written:
		assert.NoError(t, err)
	case <-timeout:
		t.Fatal("the writer blocked after the scanner stopped")
	}
}
