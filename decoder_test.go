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
	doc := "<a attr1=\"1\" attr2=\"2\" xmlns=\"https://mydomain.org\"/>"
	r := strings.NewReader(doc)
	dec := NewDecoder(r)
	var tk Token

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.Reset(doc)
		err1 := dec.NextToken(&tk)
		assert.Nil(b, err1)
		err2 := dec.NextToken(&tk)
		assert.Nil(b, err2)
	}
}

func TestDecodeStartEnd(t *testing.T) {
	// given
	doc := "<a></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))
	var t1, t2, t3 Token

	// when
	err1 := dec.NextToken(&t1)
	err2 := dec.NextToken(&t2)
	err3 := dec.NextToken(&t3)

	// then
	assert.Nil(t, err1)
	assert.Equal(t, startElement("a"), t1)
	assert.Nil(t, err2)
	assertEndElement(t, "a", t2)
	assert.Equal(t, io.EOF, err3)
}

func TestDecodeStartTextEnd(t *testing.T) {
	// given
	doc := "<a>Hello, World!</a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))
	var tk Token

	// when
	err := dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElement("a"), tk)

	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assertTextElement(t, "Hello, World!", tk)

	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assertEndElement(t, "a", tk)

	err = dec.NextToken(&tk)
	assert.Equal(t, io.EOF, err)
}

func TestDecodeStartEndWithPrefix(t *testing.T) {
	// given
	doc := "<ns1:a></ns1:a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))
	var t1, t2, t3 Token

	// when
	err1 := dec.NextToken(&t1)
	err2 := dec.NextToken(&t2)
	err3 := dec.NextToken(&t3)

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
	var t1, t2, t3 Token

	// when
	err1 := dec.NextToken(&t1)
	err2 := dec.NextToken(&t2)
	err3 := dec.NextToken(&t3)

	// then
	assert.Nil(t, err1)
	assert.Equal(t, startElement("a"), t1)
	assert.Nil(t, err2)
	assertEndElement(t, "a", t2)
	assert.Equal(t, io.EOF, err3)
}

func TestDecodeNested(t *testing.T) {
	// given
	doc := "<a attr1=\"foo\"><b attr2=\"bar\"><c attr3=\"baz\"><d attr4=\"blubb\"></d></c></b></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))
	var tk Token

	// when / then
	err := dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("a", "attr1", "foo"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("b", "attr2", "bar"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("c", "attr3", "baz"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("d", "attr4", "blubb"), tk)
}

func TestDecodeNested2(t *testing.T) {
	// given
	doc := "<a attr1=\"foo\"><b1 attr21=\"bar1\" /><c11 attr311=\"baz11\" /><d111 attr4111=\"blubb111\"></d111></a>"
	dec := NewDecoder(bufio.NewReaderSize(strings.NewReader(doc), 1024))
	var tk Token

	// when / then
	err := dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("a", "attr1", "foo"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("b1", "attr21", "bar1"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assertEndElement(t, "b1", tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("c11", "attr311", "baz11"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assertEndElement(t, "c11", tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, startElementWithAttr("d111", "attr4111", "blubb111"), tk)
	err = dec.NextToken(&tk)
	assert.Nil(t, err)
	assertEndElement(t, "d111", tk)
}

func assertTextElement(t *testing.T, text string, token Token) {
	assert.Equal(t, uint8(TokenTypeTextElement), token.Kind)
	assert.Equal(t, []byte(text), token.ByteData)
}

func assertEndElement(t *testing.T, local string, token Token) {
	assert.Equal(t, uint8(TokenTypeEndElement), token.Kind)
	assert.Equal(t, []byte(local), token.Name.Local)
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
