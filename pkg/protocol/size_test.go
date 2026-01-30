package protocol

import "testing"

func TestPayloadSize_GrowsWithData(t *testing.T) {
	base := Payload{}

	withData := Payload{
		Data: make([]byte, 100),
	}

	if withData.Size() <= base.Size() {
		t.Fatalf("expected size to grow when data is added")
	}
}

func TestPayloadSize_StringGrowth(t *testing.T) {
	p1 := Payload{Hostname: "a"}
	p2 := Payload{Hostname: "aaaaaaaaaa"} // +9 bytes

	diff := p2.Size() - p1.Size()
	if diff < 9 {
		t.Fatalf("expected size increase >= 9, got %d", diff)
	}
}

func TestPayloadSize_Invariants(t *testing.T) {
	p := Payload{}

	if p.Size() <= 0 {
		t.Fatalf("payload size must be positive")
	}

	mockPtr := 45

	p2 := p
	p2.CustomFields = map[string]any{
		"a": 1,
		"b": "text",
		"c": true,
		"d": []string{"a", "b"},
		"e": map[int]int{
			1: 1,
		},
		"f": float32(1234),
		"g": &mockPtr,
	}

	if p2.Size() <= p.Size() {
		t.Fatalf("adding custom fields should increase size")
	}
}
