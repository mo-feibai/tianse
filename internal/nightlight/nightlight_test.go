package nightlight

import (
	"encoding/hex"
	"testing"
)

// The fixtures are real registry values, not hand-made examples. The format is
// undocumented, so a fabricated blob would only prove that the parser agrees
// with itself.
const (
	// Windows 10 19045, Night Light configured but currently off.
	stateOff = "434201000A0201002A06B7F9BDCB062A2B0E1043420100C614CAF69BE8CEC1A2EE0100000000"
)

func decode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return b
}

func TestInnerPayloadFindsNestedList(t *testing.T) {
	inner, err := innerPayload(decode(t, stateOff))
	if err != nil {
		t.Fatalf("innerPayload: %v", err)
	}
	if len(inner) != 16 {
		t.Fatalf("inner payload length = %d, want 16", len(inner))
	}
	if inner[0] != 0x43 || inner[1] != 0x42 {
		t.Fatalf("inner payload does not start with the CB header: % X", inner[:4])
	}
}

func TestInnerOnReadsFieldZero(t *testing.T) {
	// The off state carries field 20 (a timestamp) and no field 0.
	inner, err := innerPayload(decode(t, stateOff))
	if err != nil {
		t.Fatalf("innerPayload: %v", err)
	}
	if innerOn(inner) {
		t.Fatal("state without field 0 must read as off")
	}

	// Field 0 as a bool is 0x02 followed by its value; Windows writes it only
	// while Night Light is on.
	on := []byte{0x43, 0x42, 0x01, 0x00, 0x02, 0x01, 0x00, 0x00}
	if !innerOn(on) {
		t.Fatal("state with field 0 must read as on")
	}
}

func TestInnerPayloadRejectsGarbage(t *testing.T) {
	if _, err := innerPayload([]byte{0x00, 0x01, 0x02, 0x03}); err == nil {
		t.Fatal("a non CompactBinary blob must not parse")
	}
	if _, err := innerPayload(decode(t, stateOff)[:12]); err == nil {
		t.Fatal("a truncated blob must not parse")
	}
}
