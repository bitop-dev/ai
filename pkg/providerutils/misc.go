package providerutils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

type IDGenerator struct {
	reader io.Reader
}

func NewIDGenerator(reader io.Reader) IDGenerator {
	return IDGenerator{reader: reader}
}

func (generator IDGenerator) NewID() (string, error) {
	reader := generator.reader
	if reader == nil {
		reader = rand.Reader
	}

	buffer := make([]byte, 16)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", err
	}

	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	return formatUUID(buffer), nil
}

func GenerateID() (string, error) {
	return NewIDGenerator(nil).NewID()
}

func formatUUID(buffer []byte) string {
	var output [36]byte
	hex.Encode(output[0:8], buffer[0:4])
	output[8] = '-'
	hex.Encode(output[9:13], buffer[4:6])
	output[13] = '-'
	hex.Encode(output[14:18], buffer[6:8])
	output[18] = '-'
	hex.Encode(output[19:23], buffer[8:10])
	output[23] = '-'
	hex.Encode(output[24:36], buffer[10:16])
	return string(output[:])
}

func NowUTC() time.Time {
	return time.Now().UTC()
}

func FormatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func ParseTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is empty")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

type NopLogger struct{}

func (NopLogger) Debugf(string, ...any) {}
func (NopLogger) Infof(string, ...any)  {}
func (NopLogger) Warnf(string, ...any)  {}
func (NopLogger) Errorf(string, ...any) {}
