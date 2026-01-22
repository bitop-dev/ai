package providerutils

import (
	"bytes"
	"testing"
	"time"
)

func TestIDGeneratorDeterministic(t *testing.T) {
	reader := bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	})

	generator := NewIDGenerator(reader)
	id, err := generator.NewID()
	if err != nil {
		t.Fatalf("expected id generation to succeed: %v", err)
	}

	const expected = "00010203-0405-4607-8809-0a0b0c0d0e0f"
	if id != expected {
		t.Fatalf("expected id %q, got %q", expected, id)
	}
}

func TestGenerateIDFormat(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("expected id generation to succeed: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("expected uuid length 36, got %d", len(id))
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("expected uuid with dashes, got %q", id)
	}
}

func TestTimestampHelpers(t *testing.T) {
	value := time.Date(2026, time.January, 22, 18, 4, 5, 123000000, time.FixedZone("UTC-2", -2*60*60))
	formatted := FormatTimestamp(value)
	parsed, err := ParseTimestamp(formatted)
	if err != nil {
		t.Fatalf("expected parse to succeed: %v", err)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("expected parsed time in UTC")
	}
	if parsed.Format(time.RFC3339Nano) != value.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("expected parsed time to match input")
	}
}

func TestNopLogger(t *testing.T) {
	logger := NopLogger{}
	logger.Debugf("debug")
	logger.Infof("info")
	logger.Warnf("warn")
	logger.Errorf("error")
}
