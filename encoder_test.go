package gosaxml

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func BenchmarkEncodeStartTokenWithNamespaceModifier(b *testing.B) {
	w := ioutil.Discard
	enc := NewEncoder(w, NewNamespaceModifier())
	token := Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local:  bs("a"),
			Prefix: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://my.org"),
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
	enc := NewEncoder(w)
	token0 := Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local:  bs("a"),
			Prefix: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local:  bs("b"),
				Prefix: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	}
	token1 := Token{
		Kind: TokenTypeEndElement,
		Name: Name{
			Local:  bs("a"),
			Prefix: bs("b"),
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
	enc := NewEncoder(w, NewNamespaceModifier())

	// when
	err := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local:  bs("a"),
			Prefix: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local:  bs("b"),
				Prefix: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})

	// then
	assert.Nil(t, err)
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\"", w.String())
}

func TestEncodeStartElementEndElement(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := NewEncoder(w, NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Prefix: bs("c"),
			Local:  bs("a"),
		},
		Attr: []Attr{{
			Name: Name{
				Prefix: bs("xmlns"),
				Local:  bs("c"),
			},
			Value: bs("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})
	err3 := enc.EncodeToken(&Token{
		Kind:     TokenTypeTextElement,
		ByteData: bs("Hello"),
	})
	err4 := enc.EncodeToken(&Token{
		Kind: TokenTypeEndElement,
		Name: Name{
			Local: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})
	err5 := enc.EncodeToken(&Token{
		Kind: TokenTypeEndElement,
		Name: Name{
			Local: bs("a"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})

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
	enc := NewEncoder(w, NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local:  bs("a"),
			Prefix: bs("ns1"),
		},
		Attr: []Attr{{
			Name: Name{
				Local:  bs("ns1"),
				Prefix: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})

	// then
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, "<a:a xmlns:a=\"https://mynamespace\"><a:b", w.String())
}

func TestEncodeTwoNestedWithRedundantNamespaceUnprefixed(t *testing.T) {
	// given
	w := &bytes.Buffer{}
	enc := NewEncoder(w, NewNamespaceModifier())

	// when
	err1 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: bs("a"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})
	err2 := enc.EncodeToken(&Token{
		Kind: TokenTypeStartElement,
		Name: Name{
			Local: bs("b"),
		},
		Attr: []Attr{{
			Name: Name{
				Local: bs("xmlns"),
			},
			Value: bs("https://mynamespace"),
		}},
	})

	// then
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.Equal(t, "<a xmlns=\"https://mynamespace\"><b", w.String())
}
