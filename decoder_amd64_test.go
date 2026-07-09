package gosaxml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDecodeTextSSEStaleAngleBracket ensures that the SSE text scanner
// does not treat a stale '<' beyond the valid buffer window (a leftover
// from a previous buffer fill) as a real match.
func TestDecodeTextSSEStaleAngleBracket(t *testing.T) {
	if !canUseSSE {
		t.Skip("SSE2+BMI1 not available")
	}
	oldAVX2 := canUseAVX2
	canUseAVX2 = false
	defer func() { canUseAVX2 = oldAVX2 }()

	d := NewDecoder(strings.NewReader("z</a>")).(*decoder)
	// Simulate a partially-filled read buffer whose stale region
	// (beyond w) still contains a '<' from a previous fill.
	copy(d.rb[:], "ab")
	d.rb[5] = '<'
	d.r, d.w = 0, 2

	var tk Token
	cntn, err := d.decodeText(&tk)
	assert.Nil(t, err)
	assert.False(t, cntn)
	assert.Equal(t, "abz", string(tk.ByteData))
}
