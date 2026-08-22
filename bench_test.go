package gosaxml_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/HBTGmbH/gosaxml"
)

// ---------------------------------------------------------------------------
// realistic corpora
// ---------------------------------------------------------------------------

// soapDoc builds a large, namespace-heavy SOAP-like document with a mix of
// attributes, short text nodes and pretty-printing whitespace.
func soapDoc(n int) string {
	var b strings.Builder
	_, _ = b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	_, _ = b.WriteString(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`xmlns:xsd="http://www.w3.org/2001/XMLSchema" ` +
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + "\n")
	_, _ = b.WriteString("  <soapenv:Header/>\n  <soapenv:Body>\n")
	_, _ = b.WriteString(`    <ns1:GetOrdersResponse xmlns:ns1="urn:example:orders:v1">` + "\n")
	for i := 0; i < n; i++ {
		_, _ = fmt.Fprintf(&b, `      <ns1:order id="ORD-%06d" currency="EUR" xsi:type="xsd:string">`+"\n", i)
		_, _ = fmt.Fprintf(&b, `        <ns1:customer number="%d">Firstname Lastname %d</ns1:customer>`+"\n", i, i)
		_, _ = fmt.Fprintf(&b, `        <ns1:createdAt>2024-11-%02dT12:%02d:00Z</ns1:createdAt>`+"\n", 1+i%28, i%60)
		_, _ = b.WriteString("        <ns1:positions>\n")
		for j := 0; j < 3; j++ {
			_, _ = fmt.Fprintf(&b, `          <ns1:position no="%d" sku="SKU-%d-%d"><ns1:qty>%d</ns1:qty>`+
				`<ns1:price>%d.99</ns1:price><ns1:desc>Some product description text</ns1:desc></ns1:position>`+"\n",
				j, i, j, j+1, 10*j+i%7)
		}
		_, _ = b.WriteString("        </ns1:positions>\n")
		_, _ = b.WriteString("      </ns1:order>\n")
	}
	_, _ = b.WriteString("    </ns1:GetOrdersResponse>\n  </soapenv:Body>\n</soapenv:Envelope>")
	return b.String()
}

// textDoc builds a document dominated by long character data.
func textDoc(n int) string {
	var b strings.Builder
	_, _ = b.WriteString("<doc>")
	para := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor " +
		"incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud " +
		"exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."
	for i := 0; i < n; i++ {
		_, _ = b.WriteString("<p>")
		_, _ = b.WriteString(para)
		_, _ = b.WriteString(para)
		_, _ = b.WriteString("</p>")
	}
	_, _ = b.WriteString("</doc>")
	return b.String()
}

// nameDoc builds a document dominated by element names and structure.
func nameDoc(n int) string {
	var b strings.Builder
	_, _ = b.WriteString("<catalogOfProducts>")
	for i := 0; i < n; i++ {
		_, _ = b.WriteString("<productEntryRecord><productIdentifier>1</productIdentifier>" +
			"<productDisplayName>2</productDisplayName>" +
			"<productShortDescription>3</productShortDescription>" +
			"<productLongDescription>4</productLongDescription>" +
			"</productEntryRecord>")
	}
	_, _ = b.WriteString("</catalogOfProducts>")
	return b.String()
}

// attrDoc builds a document dominated by attributes.
func attrDoc(n int) string {
	var b strings.Builder
	_, _ = b.WriteString("<rows>")
	for i := 0; i < n; i++ {
		_, _ = fmt.Fprintf(&b, `<row a="alpha" b="bravo" c="charlie" d="delta" e="echo" f="foxtrot" `+
			`g="golf" h="hotel" idx="%d"/>`, i)
	}
	_, _ = b.WriteString("</rows>")
	return b.String()
}

var (
	corpusSOAP = soapDoc(400)
	corpusText = textDoc(400)
	corpusName = nameDoc(2000)
	corpusAttr = attrDoc(3000)
)

// ---------------------------------------------------------------------------
// benchmarks
// ---------------------------------------------------------------------------

func benchDecode(b *testing.B, doc string) {
	b.Helper()
	r := strings.NewReader(doc)
	dec := gosaxml.NewDecoder(r)
	var tk gosaxml.Token
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(doc)
		dec.Reset(r)
		for {
			err := dec.NextToken(&tk)
			if err != nil {
				if err != io.EOF {
					b.Fatal(err)
				}
				break
			}
		}
	}
}

func benchRoundTrip(b *testing.B, doc string, ns bool) {
	b.Helper()
	r := strings.NewReader(doc)
	dec := gosaxml.NewDecoder(r)
	var enc *gosaxml.Encoder
	if ns {
		enc = gosaxml.NewEncoder(io.Discard, gosaxml.NewNamespaceModifier())
	} else {
		enc = gosaxml.NewEncoder(io.Discard)
	}
	var tk gosaxml.Token
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(doc)
		dec.Reset(r)
		enc.Reset(io.Discard)
		for {
			err := dec.NextToken(&tk)
			if err != nil {
				if err != io.EOF {
					b.Fatal(err)
				}
				break
			}
			if err = enc.EncodeToken(&tk); err != nil {
				b.Fatal(err)
			}
		}
		if err := enc.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeSOAP(b *testing.B) { benchDecode(b, corpusSOAP) }
func BenchmarkDecodeText(b *testing.B) { benchDecode(b, corpusText) }
func BenchmarkDecodeName(b *testing.B) { benchDecode(b, corpusName) }
func BenchmarkDecodeAttr(b *testing.B) { benchDecode(b, corpusAttr) }

func BenchmarkRoundTripSOAP(b *testing.B)     { benchRoundTrip(b, corpusSOAP, true) }
func BenchmarkRoundTripText(b *testing.B)     { benchRoundTrip(b, corpusText, true) }
func BenchmarkRoundTripName(b *testing.B)     { benchRoundTrip(b, corpusName, true) }
func BenchmarkRoundTripAttr(b *testing.B)     { benchRoundTrip(b, corpusAttr, true) }
func BenchmarkRoundTripSOAPNoNS(b *testing.B) { benchRoundTrip(b, corpusSOAP, false) }

// BenchmarkEncodeOnly measures the pure encoder path by pre-decoding all tokens
// into an owned slice first.
func BenchmarkEncodeOnly(b *testing.B) {
	type tok struct {
		kind byte
		name gosaxml.Name
		attr []gosaxml.Attr
		data []byte
	}
	var toks []tok
	dec := gosaxml.NewDecoder(strings.NewReader(corpusSOAP))
	var tk gosaxml.Token
	for {
		if err := dec.NextToken(&tk); err != nil {
			break
		}
		t := tok{kind: tk.Kind}
		t.name = gosaxml.Name{Local: append([]byte(nil), tk.Name.Local...), Prefix: append([]byte(nil), tk.Name.Prefix...)}
		if len(tk.Name.Prefix) == 0 {
			t.name.Prefix = nil
		}
		for _, a := range tk.Attr {
			na := gosaxml.Attr{
				Name:        gosaxml.Name{Local: append([]byte(nil), a.Name.Local...)},
				Value:       append([]byte(nil), a.Value...),
				SingleQuote: a.SingleQuote,
			}
			if len(a.Name.Prefix) > 0 {
				na.Name.Prefix = append([]byte(nil), a.Name.Prefix...)
			}
			t.attr = append(t.attr, na)
		}
		t.data = append([]byte(nil), tk.ByteData...)
		toks = append(toks, t)
	}
	var w bytes.Buffer
	enc := gosaxml.NewEncoder(&w)
	b.ReportAllocs()
	b.ResetTimer()
	var out gosaxml.Token
	for i := 0; i < b.N; i++ {
		w.Reset()
		enc.Reset(&w)
		for j := range toks {
			out.Kind = toks[j].kind
			out.Name = toks[j].name
			out.Attr = toks[j].attr
			out.ByteData = toks[j].data
			if err := enc.EncodeToken(&out); err != nil {
				b.Fatal(err)
			}
		}
		if err := enc.Flush(); err != nil {
			b.Fatal(err)
		}
		if i == 0 {
			b.SetBytes(int64(w.Len()))
		}
	}
}

// longNameDoc builds a document whose element names are far longer than the
// scalar prefix, so that the vectorized name scanner does the work.
func longNameDoc(n int) string {
	name := "ExtremelyVerboseAndDeliberatelyLongElementNameForScanning"
	var b strings.Builder
	_, _ = b.WriteString("<root>")
	for i := 0; i < n; i++ {
		_, _ = b.WriteString("<" + name + "><" + name + "Inner/></" + name + ">")
	}
	_, _ = b.WriteString("</root>")
	return b.String()
}

// indentedDoc builds a deeply indented document, i.e. one dominated by long
// runs of ignorable whitespace.
func indentedDoc(n int) string {
	var b strings.Builder
	_, _ = b.WriteString("<root>\n")
	indent := strings.Repeat(" ", 60)
	for i := 0; i < n; i++ {
		_, _ = b.WriteString(indent + "<a>\n" + indent + indent + "<b/>\n" + indent + "</a>\n")
	}
	_, _ = b.WriteString("</root>")
	return b.String()
}

var (
	corpusLongName = longNameDoc(1500)
	corpusIndented = indentedDoc(1500)
)

func BenchmarkDecodeLongNames(b *testing.B) { benchDecode(b, corpusLongName) }
func BenchmarkDecodeIndented(b *testing.B)  { benchDecode(b, corpusIndented) }
