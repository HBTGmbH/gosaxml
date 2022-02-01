[![Go Reference](https://pkg.go.dev/badge/github.com/HBTGmbH/gosaxml.svg)](https://pkg.go.dev/github.com/HBTGmbH/gosaxml) [![Go Report Card](https://goreportcard.com/badge/github.com/HBTGmbH/gosaxml)](https://goreportcard.com/report/github.com/HBTGmbH/gosaxml)

gosaxml is a streaming XML decoder and encoder, similar in interface to the `encoding/xml`, but with a focus on performance, low memory footprint and on
fixing many of the issues present in `encoding/xml` mainly related to handling of namespaces (see https://github.com/golang/go/issues/13400).

In addition to handling namespaces, gosaxml can also canonicalize and minify XML namespaces bindings in a document (with and without prefixes)
and does not repeat the prefix-less namespace declaration
on all encoded XML elements, like `encoding/xml` does.

# Get it

```shell
go get -u github.com/HBTGmbH/gosaxml
```

# Features 

* zero-allocation stream decoding of XML inputs (from `io.Reader`)
* zero-allocation stream encoding of XML elements (to `io.Writer`)
* tidying of XML namespace declarations of the encoder input

# Simple examples

## Decode and re-encode
The following example (in the form of a Go test) decodes from a given `io.Reader` and encodes the same tokens
into a provided `io.Writer`:
```go
func TestDecodeAndEncode(t *testing.T) {
	// given
	var r io.Reader = strings.NewReader(
		`<a xmlns="http://mynamespace.org">
		<b>Hi!</b>
		<c></c>
		</a>`)
	var w bytes.Buffer
	dec := gosaxml.NewDecoder(r)
	enc := gosaxml.NewEncoder(&w)

	// when
	var tk gosaxml.Token
	for {
		err := dec.NextToken(&tk)
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)

		err = enc.EncodeToken(&tk)
		assert.Nil(t, err)
	}

	// then
	assert.Equal(t,
	`<a xmlns="http://mynamespace.org"><b>Hi!</b><c/></a>`,
	w.String())
}
```