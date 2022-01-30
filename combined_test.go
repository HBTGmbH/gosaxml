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
	var tk Token

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.Reset(input)
		dec.Reset(r)
		for {
			err := dec.NextToken(&tk)
			if err != nil {
				break
			}
			err = enc.EncodeToken(&tk)
			assert.Nil(b, err)
		}
	}
}

func TestNamespacePrefixedAndUnprefixed(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<b xmlns=\"https://mynamespace\">" +
			"<c />" +
			"</b>" +
			"</ns:a>"))
	enc := NewEncoder(bb, NewNamespaceModifier())
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"<a:b>"+
		"<a:c/>"+
		"</a:b>"+
		"</a:a>", bb.String())
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
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"<a:b/>"+
		"</a:a>", bb.String())
}

func BenchmarkSameNamespaceSideBySide(b *testing.B) {
	r := strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<ns:b/>" +
			"</ns:a>" +
			"<ns1:a xmlns:ns1=\"https://mynamespace\">" +
			"<ns1:b/>" +
			"</ns:a>")
	dec := NewDecoder(r)
	enc := NewEncoder(io.Discard, NewNamespaceModifier())
	var tk Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Seek(0, io.SeekStart)
		dec.Reset(r)
		for {
			err := dec.NextToken(&tk)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}
			err = enc.EncodeToken(&tk)
		}
	}
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
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

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
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

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
			"<book category=\"children\" xmlns=\"http://mydomain.org\">" +
			"<title kind=\"title\" xmlns=\"http://mydomain.org\">Harry Potter</title>" +
			"<author>J K. Rowling</author>" +
			"<year>2005</year>" +
			"<price>29.99</price>" +
			"</book>" +
			"<book category=\"web\" xmlns=\"http://mydomain.org\">" +
			"<title kind=\"title\" xmlns=\"http://mydomain.org\">Learning XML</title>" +
			"<author>Erik T. Ray</author>" +
			"<year>2003</year>" +
			"<price>39.95</price>" +
			"</book>" +
			"</bookstore>"))
	enc := NewEncoder(bb, NewNamespaceModifier())
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<bookstore>"+
		"<book category=\"children\" xmlns=\"http://mydomain.org\">"+
		"<title kind=\"title\">Harry Potter</title>"+
		"<author>J K. Rowling</author>"+
		"<year>2005</year>"+
		"<price>29.99</price>"+
		"</book>"+
		"<book category=\"web\" xmlns=\"http://mydomain.org\">"+
		"<title kind=\"title\">Learning XML</title>"+
		"<author>Erik T. Ray</author>"+
		"<year>2003</year>"+
		"<price>39.95</price>"+
		"</book>"+
		"</bookstore>", bb.String())
}

func BenchmarkElementsAndAttributes(b *testing.B) {
	r := strings.NewReader(
		"<bookstore>" +
			"<book category=\"children\" xmlns=\"http://mydomain.org\">" +
			"<title kind=\"title\" xmlns=\"http://mydomain.org\">Harry Potter</title>" +
			"<author>J K. Rowling</author>" +
			"<year>2005</year>" +
			"<price>29.99</price>" +
			"</book>" +
			"<book category=\"web\" xmlns=\"http://mydomain.org\">" +
			"<title kind=\"title\" xmlns=\"http://mydomain.org\">Learning XML</title>" +
			"<author>Erik T. Ray</author>" +
			"<year>2003</year>" +
			"<price>39.95</price>" +
			"</book>" +
			"</bookstore>")
	dec := NewDecoder(r)
	enc := NewEncoder(io.Discard, NewNamespaceModifier())
	var tk Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Seek(0, io.SeekStart)
		dec.Reset(r)
		for {
			err := dec.NextToken(&tk)
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}
			_ = enc.EncodeToken(&tk)
		}
	}
}

func TestAttributesWithNamespace(t *testing.T) {
	// given
	input := `
<soap:Envelope
xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"
soap:encodingStyle="http://www.w3.org/2003/05/soap-encoding"></soap:Envelope>`
	dec := NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := NewEncoder(w, NewNamespaceModifier())
	var tk Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, `
<a:Envelope xmlns:a="http://www.w3.org/2003/05/soap-envelope/" a:encodingStyle="http://www.w3.org/2003/05/soap-encoding"/>`, w.String())
}

func decodeEncode(t *testing.T, dec Decoder, enc *Encoder, tk *Token) {
	for {
		err := dec.NextToken(tk)
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		err = enc.EncodeToken(tk)
		assert.Nil(t, err)
	}
}
