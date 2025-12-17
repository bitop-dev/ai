package sse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Decoder reads Server-Sent Events (SSE) and yields event payloads (the joined
// "data:" lines) as raw bytes. It is intentionally minimal for OpenAI-style SSE.
type Decoder struct {
	r   *bufio.Reader
	buf bytes.Buffer
	err error
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

// Next advances to the next event. It returns false on EOF or error. After a
// successful Next, Data returns the raw event payload (without the trailing
// newline).
func (d *Decoder) Next() bool {
	if d.err != nil {
		return false
	}
	d.buf.Reset()

	for {
		line, err := d.r.ReadString('\n')
		if err != nil {
			if err == io.EOF && d.buf.Len() > 0 {
				d.err = io.EOF
				return true
			}
			d.err = err
			return false
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// event boundary
			return true
		}

		// Ignore comments and non-data fields.
		if strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		v := strings.TrimPrefix(line, "data:")
		if strings.HasPrefix(v, " ") {
			v = strings.TrimPrefix(v, " ")
		}
		if d.buf.Len() > 0 {
			d.buf.WriteByte('\n')
		}
		d.buf.WriteString(v)
	}
}

func (d *Decoder) Data() []byte {
	if d == nil {
		return nil
	}
	return d.buf.Bytes()
}

func (d *Decoder) Err() error {
	if d == nil {
		return nil
	}
	if d.err == io.EOF {
		return nil
	}
	return d.err
}

func (d *Decoder) ExpectNoError() error {
	if err := d.Err(); err != nil {
		return fmt.Errorf("sse decode: %w", err)
	}
	return nil
}
