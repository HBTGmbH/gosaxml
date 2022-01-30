package gosaxml

import (
	"bytes"
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
	dec := NewDecoder(strings.NewReader(input))
	w := &bytes.Buffer{}
	nm := NewNamespaceModifier()
	enc := NewEncoder(w, nm)
	var tk Token

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
		if tk.Kind == TokenTypeStartElement &&
			bytes.Equal(tk.Name.Local, bs("GetPrice")) &&
			bytes.Equal(nm.NamespaceOfToken(&tk), bs(pricesNamespace)) {

			// inject '\n    <m:Item>Apples</m:Item>' here.
			// We do not know the concrete prefix to use, but we _do_ know the namespace
			// that we want the new element to reside in (this is usually known in advance).
			// So, we can add a Token of kind TokenTypeStartElement with an "xmlns" attribute
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

func addEndElement(t *testing.T, enc *Encoder) {
	err := enc.EncodeToken(&Token{
		Kind: TokenTypeEndElement,
	})
	assert.Nil(t, err)
}

func addStartElement(t *testing.T, enc *Encoder, local, namespace string) {
	err := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: bs(local),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs(namespace),
		}},
	})
	assert.Nil(t, err)
}

func addTextToken(t *testing.T, enc *Encoder, text string) {
	err := enc.EncodeToken(&Token{
		Kind:     TokenTypeTextElement,
		ByteData: bs(text),
	})
	assert.Nil(t, err)
}
