package gosaxml

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func BenchmarkNamespaceAlias1Level(b *testing.B) {
	input := "<ns:a xmlns:ns=\"https://mynamespace\"/>"
	r := strings.NewReader(input)
	dec := NewDecoder(r)
	enc := NewEncoder(io.Discard, NewNamespaceModifier())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.Reset(input)
		dec.Reset(r)
		for {
			tk, err := dec.NextToken()
			if err != nil {
				break
			}
			err = enc.EncodeToken(tk)
			assert.Nil(b, err)
		}
	}
}

func TestNamespaceAlias1Level(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<ns1:b xmlns:ns1=\"https://mynamespace\">" +
			"</ns1:b>" +
			"</ns:a>"))
	enc := NewEncoder(bb, NewNamespaceModifier())

	// when
	decodeEncode(t, dec, enc)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"<a:b/>"+
		"</a:a>", bb.String())
}

func TestSameNamespaceSideBySide(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<ns:b/>" +
			"</ns:a>" +
			"<ns1:a xmlns:ns1=\"https://mynamespace\">" +
			"<ns1:b/>" +
			"</ns:a>"))
	enc := NewEncoder(bb, NewNamespaceModifier())

	// when
	decodeEncode(t, dec, enc)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"<a:b/>"+
		"</a:a>"+
		"<a:a xmlns:a=\"https://mynamespace\">"+
		"<a:b/>"+
		"</a:a>", bb.String())
}

func TestBeginTextEnd(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"Hello, World!" +
			"</ns:a>"))
	enc := NewEncoder(bb, NewNamespaceModifier())

	// when
	decodeEncode(t, dec, enc)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"Hello, World!"+
		"</a:a>", bb.String())
}

func TestElementsAndAttributes(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := NewDecoder(strings.NewReader(
		"<bookstore>" +
			"<book category=\"children\">" +
			"<title>Harry Potter</title>" +
			"<author>J K. Rowling</author>" +
			"<year>2005</year>" +
			"<price>29.99</price>" +
			"</book>" +
			"<book category=\"web\">" +
			"<title>Learning XML</title>" +
			"<author>Erik T. Ray</author>" +
			"<year>2003</year>" +
			"<price>39.95</price>" +
			"</book>" +
			"</bookstore>"))
	enc := NewEncoder(bb)

	// when
	decodeEncode(t, dec, enc)

	// then
	assert.Equal(t, "<bookstore>"+
		"<book category=\"children\">"+
		"<title>Harry Potter</title>"+
		"<author>J K. Rowling</author>"+
		"<year>2005</year>"+
		"<price>29.99</price>"+
		"</book>"+
		"<book category=\"web\">"+
		"<title>Learning XML</title>"+
		"<author>Erik T. Ray</author>"+
		"<year>2003</year>"+
		"<price>39.95</price>"+
		"</book>"+
		"</bookstore>", bb.String())
}

func decodeEncode(t *testing.T, dec Decoder, enc *Encoder) {
	for {
		tk, err := dec.NextToken()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		err = enc.EncodeToken(tk)
		assert.Nil(t, err)
	}
}
