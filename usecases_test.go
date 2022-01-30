package gosaxml_test

import (
	"bytes"
	"github.com/HBTGmbH/gosaxml"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func TestInjectElementInSOAPBody(t *testing.T) {
	// given
	input := `
<soap:Envelope
xmlns:soap="http://www.w3.org/2003/05/soap-envelope/"
soap:encodingStyle="http://www.w3.org/2003/05/soap-encoding">
<soap:Body>
  <m:GetPrice xmlns:m="https://www.w3schools.com/prices">
    <!-- we want to add a <m:Item>Apples</m:Item> here -->
  </m:GetPrice>
</soap:Body>
</soap:Envelope>`
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	nm := gosaxml.NewNamespaceModifier()
	enc := gosaxml.NewEncoder(w, nm)
	var tk gosaxml.Token

	// when
	for {
		err := dec.NextToken(&tk)
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)

		err = enc.EncodeToken(&tk)
		assert.Nil(t, err)

		// check if this is "https://www.w3schools.com/prices":GetPrice
		pricesNamespace := "https://www.w3schools.com/prices"
		if tk.Kind == gosaxml.TokenTypeStartElement &&
			bytes.Equal(tk.Name.Local, []byte("GetPrice")) &&
			bytes.Equal(nm.NamespaceOfToken(&tk), []byte(pricesNamespace)) {

			// inject '\n    <m:Item>Apples</m:Item>' here.
			// We do not know the concrete prefix to use, but we _do_ know the namespace
			// that we want the new element to reside in (this is usually known in advance).
			// So, we can add a gosaxml.Token of kind gosaxml.TokenTypeStartElement with an "xmlns" attribute
			// which the NamespaceModifier will then translate to the already known prefix
			// for that namespace.
			addTextToken(t, enc, "\n    ")
			addStartElement(t, enc, "Item", pricesNamespace)
			addTextToken(t, enc, "Apples")
			addEndElement(t, enc)
		}
	}

	// then
	assert.Equal(t, `
<a:Envelope xmlns:a="http://www.w3.org/2003/05/soap-envelope/" a:encodingStyle="http://www.w3.org/2003/05/soap-encoding">
<a:Body>
  <b:GetPrice xmlns:b="https://www.w3schools.com/prices">
    <b:Item>Apples</b:Item>
    
  </b:GetPrice>
</a:Body>
</a:Envelope>`, w.String())
}

func addEndElement(t *testing.T, enc *gosaxml.Encoder) {
	err := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeEndElement,
	})
	assert.Nil(t, err)
}

func addStartElement(t *testing.T, enc *gosaxml.Encoder, local, namespace string) {
	err := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local: []byte(local),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte(namespace),
		}},
	})
	assert.Nil(t, err)
}

func addTextToken(t *testing.T, enc *gosaxml.Encoder, text string) {
	err := enc.EncodeToken(&gosaxml.Token{
		Kind:     gosaxml.TokenTypeTextElement,
		ByteData: []byte(text),
	})
	assert.Nil(t, err)
}
