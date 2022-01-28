package gosaxml

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func BenchmarkNextToken(b *testing.B) {
	// given
	doc := "<a xmlns=\"https://mydomain.org\"/>"
	r := strings.NewReader(doc)
	dec := NewDecoder(r)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.Reset(doc)
		_, err1 := dec.NextToken()
		assert.Nil(b, err1)
		_, err2 := dec.NextToken()
		assert.Nil(b, err2)
	}
}

func TestDecodeStartEnd(t *testing.T) {
	// given
	doc := "<a></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when
	t1, err1 := dec.NextToken()
	t2, err2 := dec.NextToken()
	_, err3 := dec.NextToken()

	// then
	assert.Nil(t, err1)
	assert.Equal(t, startElement("a"), t1)
	assert.Nil(t, err2)
	assert.Equal(t, endElement("a"), t2)
	assert.Equal(t, io.EOF, err3)
}

func TestDecodeStartTextEnd(t *testing.T) {
	// given
	doc := "<a>Hello, World!</a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when
	token, err := dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElement("a"), token)

	token, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, textElement("Hello, World!"), token)

	token, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, endElement("a"), token)

	_, err = dec.NextToken()
	assert.Equal(t, io.EOF, err)
}

func TestDecodeStartEndWithPrefix(t *testing.T) {
	// given
	doc := "<ns1:a></ns1:a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when
	t1, err1 := dec.NextToken()
	t2, err2 := dec.NextToken()
	_, err3 := dec.NextToken()

	// then
	assert.Nil(t, err1)
	assert.Equal(t, startElementWithPrefix("ns1", "a"), t1)
	assert.Nil(t, err2)
	assert.Equal(t, endElementWithPrefix("ns1", "a"), t2)
	assert.Equal(t, io.EOF, err3)
}

func TestDecodeStartEndImplicit(t *testing.T) {
	// given
	doc := "<a/>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when
	t1, err1 := dec.NextToken()
	t2, err2 := dec.NextToken()
	_, err3 := dec.NextToken()

	// then
	assert.Nil(t, err1)
	assert.Equal(t, startElement("a"), t1)
	assert.Nil(t, err2)
	assert.Equal(t, endElement("a"), t2)
	assert.Equal(t, io.EOF, err3)
}

func TestDecodeNested(t *testing.T) {
	// given
	doc := "<a attr1=\"foo\"><b attr2=\"bar\"><c attr3=\"baz\"><d attr4=\"blubb\"></d></c></b></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when / then
	tk, err := dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("a", "attr1", "foo"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("b", "attr2", "bar"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("c", "attr3", "baz"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("d", "attr4", "blubb"), tk)
}

func TestDecodeNested2(t *testing.T) {
	// given
	doc := "<a attr1=\"foo\"><b1 attr21=\"bar1\" /><c11 attr311=\"baz11\" /><d111 attr4111=\"blubb111\"></d111></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))

	// when / then
	tk, err := dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("a", "attr1", "foo"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("b1", "attr21", "bar1"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, endElement("b1"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("c11", "attr311", "baz11"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, endElement("c11"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("d111", "attr4111", "blubb111"), tk)
	tk, err = dec.NextToken()
	assert.Nil(t, err)
	assert.Equal(t, endElement("d111"), tk)
}

func textElement(text string) Token {
	return Token{
		Kind:     TokenTypeTextElement,
		ByteData: []byte(text),
	}
}

func endElement(local string) Token {
	return Token{
		Kind: TokenTypeEndElement,
		Name: Name{
			Local: []byte(local),
		},
	}
}

func startElement(local string) Token {
	return Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: []byte(local),
		},
		Attr: []Attr{},
	}
}

func startElementWithPrefix(prefix, local string) Token {
	return Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Prefix: []byte(prefix),
			Local:  []byte(local),
		},
		Attr: []Attr{},
	}
}

func startElementWithAttr(local string, attrName string, attrValue string) Token {
	return Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: []byte(local),
		},
		Attr: []Attr{
			{
				Name: Name{
					Local: bs(attrName),
				},
				Value: bs(attrValue),
			},
		},
	}
}

func endElementWithPrefix(prefix, local string) Token {
	return Token{
		Kind: TokenTypeEndElement,
		Name: Name{
			Prefix: []byte(prefix),
			Local:  []byte(local),
		},
	}
}
