package gosaxml_test

import (
	"bytes"
	"github.com/HBTGmbH/gosaxml"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"testing"
)

var startNameRunes = []rune(":-_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var restNameRunes = []rune("0123456789-_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var stringRunes = []rune("/:+*#.!§$%&/[]=?`´'0123456789-_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var textRunes = []rune("\"/:+*#'.!§$%&[]=?`´'0123456789-_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var everythingRunes = []rune("<> \t\n\r\"/:+*#'.!§$%&[]=?`´'0123456789-_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randGarbage(r *rand.Rand) string {
	c := r.Intn(8000)
	b := make([]rune, c)
	for i := 0; i < c; i++ {
		b[i] = everythingRunes[r.Intn(len(everythingRunes))]
	}
	return string(b)
}

func randName(r *rand.Rand) string {
	c := 1 + r.Intn(10)
	b := make([]rune, c)
	b[0] = startNameRunes[r.Intn(len(startNameRunes))]
	for i := 1; i < c; i++ {
		b[i] = restNameRunes[r.Intn(len(restNameRunes))]
	}
	return string(b)
}

func randText(r *rand.Rand) string {
	c := 1 + r.Intn(255)
	b := make([]rune, c)
	for i := 0; i < c; i++ {
		b[i] = textRunes[r.Intn(len(textRunes))]
	}
	return string(b)
}

func randString(r *rand.Rand) string {
	c := r.Intn(30)
	b := make([]rune, c)
	for i := 0; i < c; i++ {
		b[i] = stringRunes[r.Intn(len(stringRunes))]
	}
	return string(b)
}

func buildElement(i int, b *bytes.Buffer, r *rand.Rand, lastOpen bool) bool {
	switch i {
	case 0:
		if lastOpen {
			_, _ = b.WriteString(">")
		}
		name := randName(r)
		_, _ = b.WriteString("<")
		_, _ = b.WriteString(name)
		numAttrs := r.Intn(10)
		for j := 0; j < numAttrs; j++ {
			_, _ = b.WriteString(" ")
			buildAttribute(b, r)
		}
		ended := buildElement(r.Intn(2), b, r, true)
		if !ended {
			_, _ = b.WriteString("</")
			_, _ = b.WriteString(name)
			_, _ = b.WriteString(">")
		}
		return false
	case 1:
		if lastOpen {
			_, _ = b.WriteString(">")
		}
		_, _ = b.WriteString(randText(r))
		return false
	default:
		_, _ = b.WriteString("/>")
		return true
	}
}

func buildAttribute(b *bytes.Buffer, r *rand.Rand) {
	name := randName(r)
	value := randString(r)
	randName(r)
	_, _ = b.WriteString(name)
	_, _ = b.WriteString("=\"")
	_, _ = b.WriteString(value)
	_, _ = b.WriteString("\"")
}

func TestFuzz(t *testing.T) {
	// given
	s1 := rand.NewSource(123456789)
	r := rand.New(s1)
	n := 100000

	for i := 0; i < n; i++ {
		b := &bytes.Buffer{}
		buildElement(0, b, r, false)
		xml := b.String()
		reader := bytes.NewReader(b.Bytes())
		dec := gosaxml.NewDecoder(reader)
		w := &bytes.Buffer{}
		modifier := gosaxml.NewNamespaceModifier()
		enc := gosaxml.NewEncoder(w, modifier)
		var tk gosaxml.Token

		// when
		for {
			err := dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			assert.Nil(t, err)
			modifier.NamespaceOfToken(&tk)
			err = enc.EncodeToken(&tk)
			assert.Nil(t, err)
		}
		assert.Nil(t, enc.Flush())

		// then
		assert.Equal(t, xml, w.String())
	}
}

func TestFuzzNoPanic(t *testing.T) {
	// given
	s1 := rand.NewSource(123456789)
	r := rand.New(s1)
	n := 100000

	for i := 0; i < n; i++ {
		xml := randGarbage(r)
		reader := bytes.NewReader([]byte(xml))
		dec := gosaxml.NewDecoder(reader)
		w := &bytes.Buffer{}
		modifier := gosaxml.NewNamespaceModifier()
		enc := gosaxml.NewEncoder(w, modifier)
		var tk gosaxml.Token

		// when
		for {
			err := dec.NextToken(&tk)
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			modifier.NamespaceOfToken(&tk)
			err = enc.EncodeToken(&tk)
			if err != nil {
				break
			}
		}
		assert.Nil(t, enc.Flush())
	}
}
