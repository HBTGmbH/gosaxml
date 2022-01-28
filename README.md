gosaxml is a streaming XML decoder and encoder, similar in interface to the `encoding/xml`, but with a focus on performance, low memory footprint and on
fixing many of the issues present in `encoding/xml` mainly related to handling of namespaces (see https://github.com/golang/go/issues/13400).

In addition to handling namespaces, gosaxml can also canonicalize and minify XML namespaces bindings in a document (with and without prefixes)
and does not repeat the prefix-less namespace declaration
on all encoded XML elements, like `encoding/xml` does.

# Get it

```shell
go get -u github.com/HBTGmbH/gosaxml@v0.0.1
```

# Features 

* zero-allocation stream decoding of XML inputs (from `io.Reader`)
* low-allocation stream encoding of XML elements (to `io.Writer`)
* tidying of XML namespace declarations of the encoder input

# Simple example

## Decode and re-encode
The following example decodes from a given `io.Reader` and encodes the same tokens
into a provided `io.Writer`:
```go
var r io.Reader = ...
var w io.Writer = ...
dec := gosaxml.NewDecoder(r)
enc := gosaxml.NewEncoder(w)
for {
	// decode the next token
	tk, err := dec.NextToken()
	if err == io.EOF {
		// io.EOF means end-of-file, so we are done
		break
	} else if err != nil {
		// any other error is a real error
		panic(err)
	}
	
	// encode the same token
	err = enc.EncodeToken(tk)
	if err != nil {
		panic(err)
	}
}
```