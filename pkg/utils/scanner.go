package utils

import (
	"bufio"
	"bytes"
	"io"
)

// MaxOutputLine is the scanner's buffer size. NewOutputScanner splits a longer
// line into pieces of this size, so one oversized line can't stop the scanner
// and the output after it is still parsed.
const MaxOutputLine = 1024 * 1024

// NewOutputScanner returns a line scanner for a child process's output. It
// ends a line at "\n", and drops text that a carriage return redraws, so a
// progress spinner such as the one `snap install` prints yields only its last
// frame instead of one line that grows for as long as the spinner runs.
func NewOutputScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxOutputLine)
	scanner.Split(splitLongLines(ScanLinesCollapseCR, MaxOutputLine))
	return scanner
}

// ScanLinesCollapseCR is bufio.ScanLines for terminal-style output. Text
// before a lone "\r" is discarded, the way a terminal overwrites it, and any
// run of "\r" before "\n" is stripped from the line.
func ScanLinesCollapseCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Redrawn text is skipped here rather than returned as a nil token, because
	// bufio.Scanner stops at EOF after a nil token and would lose what follows.
	start := 0
	for {
		i := bytes.IndexAny(data[start:], "\r\n")
		if i < 0 {
			break
		}
		i += start
		if data[i] == '\n' {
			return i + 1, data[start:i], nil
		}
		j := i
		for j < len(data) && data[j] == '\r' {
			j++
		}
		if j < len(data) && data[j] == '\n' {
			return j + 1, data[start:i], nil
		}
		if j == len(data) {
			if atEOF {
				return j, data[start:i], nil
			}
			// Wait for the next byte to tell a lone "\r" from "\r\n".
			return start, nil, nil
		}
		// A lone "\r": the text before it is redrawn, so drop it.
		start = j
	}
	if atEOF {
		return len(data), data[start:], nil
	}
	return start, nil, nil
}

// splitLongLines wraps split so a line that fills a max-byte buffer is
// returned in max-byte pieces. Without it the scanner stops with
// bufio.ErrTooLong.
func splitLongLines(split bufio.SplitFunc, max int) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := split(data, atEOF)
		if err == nil && advance == 0 && token == nil && len(data) >= max {
			return len(data), data, nil
		}
		return advance, token, err
	}
}

// DrainOutput reads r to the end and discards it. Call it after a scanner
// loop, so a child process never blocks writing to a full pipe if the scanner
// stopped early with an error.
func DrainOutput(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
}
