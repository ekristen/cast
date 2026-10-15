package utils

import (
	"bufio"
	"bytes"
	"io"
)

// MaxOutputLine is the scanner's buffer size, so NewOutputScanner returns
// lines up to about this long, counting the line ending. A longer line stops
// the scanner with bufio.ErrTooLong, and the caller drains the rest.
const MaxOutputLine = 1024 * 1024

// NewOutputScanner returns a line scanner for a child process's output. It
// ends a line at "\n" or "\r", so progress spinners that redraw a line with
// carriage returns, such as the one `snap install` prints, become short lines
// instead of one line that grows for as long as the spinner runs.
func NewOutputScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxOutputLine)
	scanner.Split(ScanLinesOrCR)
	return scanner
}

// ScanLinesOrCR is bufio.ScanLines that also ends a line at a lone "\r".
func ScanLinesOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\r' {
			// A "\r\n" pair ends one line. Wait for the next byte to tell.
			if i+1 == len(data) && !atEOF {
				return 0, nil, nil
			}
			if i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
		}
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// DrainOutput reads r to the end and discards it. Call it after a scanner
// loop, so a child process never blocks writing to a full pipe after the
// scanner has stopped early with an error.
func DrainOutput(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
}
