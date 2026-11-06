package gosaxml

import (
	"strings"
	"testing"
)

// TestDecodeTextIgnoresStaleInput makes sure the text scanner never looks at
// bytes beyond the valid input window, which may still hold leftovers from a
// previous buffer fill.
func TestDecodeTextIgnoresStaleInput(t *testing.T) {
	d := NewDecoder(strings.NewReader("z</a>")).(*decoder)
	copy(d.rb[:], "ab")
	d.rb[5] = '<'
	d.rb[6] = 'q'
	d.r, d.w = 0, 2

	var tk Token
	cntn, err := d.decodeText(&tk)
	if err != nil {
		t.Fatal(err)
	}
	if cntn {
		t.Fatal("expected a text token")
	}
	if got := string(tk.ByteData); got != "abz" {
		t.Fatalf("got %q, want %q", got, "abz")
	}
}

// TestAppendFastSpansBlockBoundary exercises the fixed-size copy right at the
// end of the input window, where it reads into the read buffer's padding.
func TestAppendFastSpansBlockBoundary(t *testing.T) {
	for n := 0; n <= overcopy+8; n++ {
		d := NewDecoder(strings.NewReader("")).(*decoder)
		for i := 0; i < n; i++ {
			d.rb[readBufferSize-n+i] = byte('a' + i%26)
		}
		d.r, d.w = readBufferSize-n, readBufferSize
		src := d.rb[d.r:d.w]
		want := string(src)
		if !d.appendFast(src) {
			d.appendFromInputSlow(src)
		}
		if got := string(d.bb); got != want {
			t.Fatalf("n=%d: got %q, want %q", n, got, want)
		}
	}
}

// TestAttrSlotReuseClearsPrefix guards the decoder's reuse of attribute slots
// without zeroing them: every field of an Attr, in particular a Name.Prefix
// left over from a previous element, has to be overwritten.
func TestAttrSlotReuseClearsPrefix(t *testing.T) {
	d := NewDecoder(strings.NewReader(
		`<r><a p:x="1" q:y="2"/><b x="3" y="4"/></r>`)).(*decoder)
	var tk Token
	var seen []string
	for {
		if err := d.NextToken(&tk); err != nil {
			break
		}
		if tk.Kind != TokenTypeStartElement {
			continue
		}
		for i := range tk.Attr {
			a := &tk.Attr[i]
			seen = append(seen, string(a.Name.Prefix)+"|"+string(a.Name.Local)+"="+string(a.Value))
			if len(tk.Name.Local) == 1 && tk.Name.Local[0] == 'b' && a.Name.Prefix != nil {
				t.Fatalf("attribute %q kept a stale prefix %q", a.Name.Local, a.Name.Prefix)
			}
		}
	}
	want := []string{"p|x=1", "q|y=2", "|x=3", "|y=4"}
	if len(seen) != len(want) {
		t.Fatalf("got %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("got %v, want %v", seen, want)
		}
	}
}
