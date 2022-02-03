package gosaxml_test

import (
	"bytes"
	"github.com/HBTGmbH/gosaxml"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func BenchmarkEncodeStartTokenWithNamespaceModifier(b *testing.B) {
	w := ioutil.Discard
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())
	token := gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local:  []byte("a"),
			Prefix: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://my.org"),
		}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Reset(w)
		err := enc.EncodeToken(&token)
		assert.Nil(b, err)
	}
}

func BenchmarkEncode(b *testing.B) {
	w := ioutil.Discard
	enc := gosaxml.NewEncoder(w)
	token0 := gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local:  []byte("a"),
			Prefix: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local:  []byte("b"),
				Prefix: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	}
	token1 := gosaxml.Token{
		Kind: gosaxml.TokenTypeEndElement,
		Name: gosaxml.Name{
			Local:  []byte("a"),
			Prefix: []byte("b"),
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		enc.Reset(w)
		err := enc.EncodeToken(&token0)
		assert.Nil(b, err)
		err = enc.EncodeToken(&token1)
		assert.Nil(b, err)
	}
}

func TestEncodeStartElement(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())

	// when
	err := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local:  []byte("a"),
			Prefix: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local:  []byte("b"),
				Prefix: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	assert.Nil(t, enc.Flush())

	// then
	assert.Nil(t, err)
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\"", w.String())
}

func TestEncodeStartElementEndElement(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Prefix: []byte("c"),
			Local:  []byte("a"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Prefix: []byte("xmlns"),
				Local:  []byte("c"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	err3 := enc.EncodeToken(&gosaxml.Token{
		Kind:     gosaxml.TokenTypeTextElement,
		ByteData: []byte("Hello"),
	})
	err4 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeEndElement,
		Name: gosaxml.Name{
			Local: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	err5 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeEndElement,
		Name: gosaxml.Name{
			Local: []byte("a"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	assert.Nil(t, enc.Flush())

	// then
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Nil(t, err3)
	assert.Nil(t, err4)
	assert.Nil(t, err5)
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\"><a:b>Hello</a:b></a:a>", w.String())
}

func TestEncodeTwoNestedWithRedundantNamespace(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local:  []byte("a"),
			Prefix: []byte("ns1"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local:  []byte("ns1"),
				Prefix: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	assert.Nil(t, enc.Flush())

	// then
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\"><a:b", w.String())
}

func TestEncodeTwoNestedWithRedundantNamespaceUnprefixed(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := gosaxml.NewEncoder(w, gosaxml.NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local: []byte("a"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&gosaxml.Token{
		Kind: gosaxml.TokenTypeStartElement,
		Name: gosaxml.Name{
			Local: []byte("b"),
		},
		Attr: []gosaxml.Attr{{
			Name: gosaxml.Name{
				Local: []byte("xmlns"),
			},
			Value: []byte("https://mynamespace"),
		}},
	})
	assert.Nil(t, enc.Flush())

	// then
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, "<a xmlns=\"https://mynamespace\"><b", w.String())
}
