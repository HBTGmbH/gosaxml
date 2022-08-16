package gosaxml_test

import (
	"bytes"
	"github.com/HBTGmbH/gosaxml"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func BenchmarkNamespaceAlias1Level(b *testing.B) {
	input := "<ns:a xmlns:ns=\"https://mynamespace\"/>"
	r := strings.NewReader(input)
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

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
		assert.Nil(b, enc.Flush())
	}
}

func TestNamespacePrefixedAndUnprefixed(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<b xmlns=\"https://mynamespace\">" +
			"<c />" +
			"</b>" +
			"</ns:a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

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
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<ns1:b xmlns:ns1=\"https://mynamespace\">" +
			"</ns1:b>" +
			"</ns:a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

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
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := r.Seek(0, io.SeekStart)
		assert.Nil(b, err)
		dec.Reset(r)
		for {
			err = dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			assert.Nil(b, err)
			err = enc.EncodeToken(&tk)
			assert.Nil(b, err)
		}
		assert.Nil(b, enc.Flush())
	}
}

func TestSameNamespaceSideBySide(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"<ns:b/>" +
			"</ns:a>" +
			"<ns1:a xmlns:ns1=\"https://mynamespace\">" +
			"<ns1:b/>" +
			"</ns:a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

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
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<ns:a xmlns:ns=\"https://mynamespace\">" +
			"Hello, World!" +
			"</ns:a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\">"+
		"Hello, World!"+
		"</a:a>", bb.String())
}

func TestAttributeWithSingleQuote(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<a attr1='https://mynam\"espace'>" +
			"Hello, World!" +
			"</a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a attr1='https://mynam\"espace'>"+
		"Hello, World!"+
		"</a>", bb.String())
}

func TestAttributeWithDoubleQuote(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := gosaxml.NewDecoder(strings.NewReader(
		"<a attr1=\"https://mynam'espace\">" +
			"Hello, World!" +
			"</a>"))
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a attr1=\"https://mynam'espace\">"+
		"Hello, World!"+
		"</a>", bb.String())
}

func TestElementsAndAttributes(t *testing.T) {
	// given
	bb := &bytes.Buffer{}
	dec := gosaxml.NewDecoder(strings.NewReader(
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
	enc := gosaxml.NewEncoder(bb, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

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
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := r.Seek(0, io.SeekStart)
		assert.Nil(b, err)
		dec.Reset(r)
		for {
			err = dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			assert.Nil(b, err)
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
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, `<a:Envelope xmlns:a="http://www.w3.org/2003/05/soap-envelope/" a:encodingStyle="http://www.w3.org/2003/05/soap-encoding"/>`, w.String())
}

func TestSOAP(t *testing.T) {
	// given
	input := `
<soap:Envelope
xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"
soap:encodingStyle="http://www.w3.org/2003/05/soap-encoding">
<soap:Body>
  <m:GetPrice xmlns:m="https://www.w3schools.com/prices">
    <m:Item>Apples</m:Item>
  </m:GetPrice>
</soap:Body>
</soap:Envelope>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a:Envelope xmlns:a=\"http://www.w3.org/2003/05/soap-envelope/\" "+
		"a:encodingStyle=\"http://www.w3.org/2003/05/soap-encoding\">"+
		"<a:Body>"+
		"<b:GetPrice xmlns:b=\"https://www.w3schools.com/prices\">"+
		"<b:Item>Apples</b:Item>"+
		"</b:GetPrice>"+
		"</a:Body>"+
		"</a:Envelope>", w.String())
}

func TestAttributesWithPrefixesPreserve(t *testing.T) {
	// given
	input := `
<ns1:a xmlns:ns1="http://ns1" ns1:attr1="val1" ns2:attr2="val2" xmlns:ns2="http://ns2">
<ns1:b>
  <b:c xmlns:b="http://ns2" ns2:attr3="val3">
    <b:d ns1:attr4="val4">Test</b:d>
  </b:c>
</ns1:b>
</ns1:a>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	namespaceModifier := gosaxml.NewNamespaceModifier()
	namespaceModifier.PreserveOriginalPrefixes = true
	enc := gosaxml.NewEncoder(w, namespaceModifier)
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<ns1:a xmlns:ns1=\"http://ns1\" ns1:attr1=\"val1\" ns2:attr2=\"val2\" xmlns:ns2=\"http://ns2\">"+
		"<ns1:b>"+
		"<ns2:c ns2:attr3=\"val3\">"+
		"<ns2:d ns1:attr4=\"val4\">Test</ns2:d>"+
		"</ns2:c>"+
		"</ns1:b>"+
		"</ns1:a>", w.String())
}

func TestAttributesWithPrefixes(t *testing.T) {
	// given
	input := `
<ns1:a xmlns:ns1="http://ns1" ns1:attr1="val1" ns2:attr2="val2" xmlns:ns2="http://ns2">
<ns1:b>
  <b:c xmlns:b="http://ns2" ns2:attr3="val3">
    <b:d ns1:attr4="val4">Test</b:d>
  </b:c>
</ns1:b>
</ns1:a>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<a:a xmlns:a=\"http://ns1\" a:attr1=\"val1\" b:attr2=\"val2\" xmlns:b=\"http://ns2\">"+
		"<a:b>"+
		"<b:c b:attr3=\"val3\">"+
		"<b:d a:attr4=\"val4\">Test</b:d>"+
		"</b:c>"+
		"</a:b>"+
		"</a:a>", w.String())
}

func TestProcInst(t *testing.T) {
	// given
	input := `
<?xml version="1.0"?>
<ns1:a xmlns:ns1="http://ns1" ns1:attr1="val1"></ns1:a>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<?xml version=\"1.0\"?>"+
		"<a:a xmlns:a=\"http://ns1\" a:attr1=\"val1\"/>", w.String())
}

func TestPreserveWhitespace(t *testing.T) {
	// given
	input := `
<?xml version="1.0"?>
<a xml:space="preserve">
<b attr1=" value ">  significantWhitespace  </b>
</a>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<?xml version=\"1.0\"?>"+
		"<a xml:space=\"preserve\">\n"+
		"<b attr1=\" value \">  significantWhitespace  </b>\n"+
		"</a>", w.String())
}

func TestInsignificantWhitespace(t *testing.T) {
	// given
	input := `
<?xml    version    =   "1.0"  encoding  =   "utf-8"   ?>
<a   xml:space  =  "preserve" >
</a  >`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	// when
	decodeEncode(t, dec, enc, &tk)

	// then
	assert.Equal(t, "<?xml version    =   \"1.0\"  encoding  =   \"utf-8\"?>"+
		"<a xml:space=\"preserve\">\n</a>", w.String())
}

func BenchmarkLotsOfText(b *testing.B) {
	r := strings.NewReader(
		`<a>Convert multiple numbers to strings and do something with them for as long as it takes</a>`)
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := r.Seek(0, io.SeekStart)
		assert.Nil(b, err)
		dec.Reset(r)
		for {
			err = dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			assert.Nil(b, err)
			err = enc.EncodeToken(&tk)
			assert.Nil(b, err)
		}
	}
}

func BenchmarkWithWhitespaceInAttributes(b *testing.B) {
	r := strings.NewReader(
		`<a         a        =       "test"         >
        </a   >`)
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := r.Seek(0, io.SeekStart)
		assert.Nil(b, err)
		dec.Reset(r)
		for {
			err = dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			assert.Nil(b, err)
			err = enc.EncodeToken(&tk)
			assert.Nil(b, err)
		}
	}
}

func decodeEncode(t *testing.T, dec gosaxml.Decoder, enc *gosaxml.Encoder, tk *gosaxml.Token) {
	for {
		err := dec.NextToken(tk)
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)
		err = enc.EncodeToken(tk)
		assert.Nil(t, err)
	}
	assert.Nil(t, enc.Flush())
}
