package gosaxml_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/HBTGmbH/gosaxml"
	"github.com/stretchr/testify/assert"
)

func collectTokens(t *testing.T, input string) ([]string, error) {
	t.Helper()
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	var tk gosaxml.Token
	var tokens []string
	for {
		err := dec.NextToken(&tk)
		if err == io.EOF {
			return tokens, nil
		}
		if err != nil {
			return tokens, err
		}
		switch tk.Kind {
		case gosaxml.TokenTypeStartElement:
			tokens = append(tokens, "start:"+string(tk.Name.Local))
		case gosaxml.TokenTypeEndElement:
			tokens = append(tokens, "end:"+string(tk.Name.Local))
		case gosaxml.TokenTypeTextElement:
			tokens = append(tokens, "text:"+string(tk.ByteData))
		case gosaxml.TokenTypeCharData:
			tokens = append(tokens, "chardata:"+string(tk.ByteData))
		case gosaxml.TokenTypeProcInst:
			tokens = append(tokens, "pi:"+string(tk.Name.Local))
		}
	}
}

func roundTrip(t *testing.T, input string) (string, error) {
	t.Helper()
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	var w bytes.Buffer
	enc := gosaxml.NewEncoder(&w, gosaxml.NewNamespaceModifier())
	var tk gosaxml.Token
	for {
		err := dec.NextToken(&tk)
		if err == io.EOF {
			break
		}
		if err != nil {
			return w.String(), err
		}
		err = enc.EncodeToken(&tk)
		if err != nil {
			return w.String(), err
		}
	}
	err := enc.Flush()
	return w.String(), err
}

func nestedDocument(depth int) string {
	var b strings.Builder
	for i := 0; i < depth; i++ {
		_, _ = fmt.Fprintf(&b, "<e%d>", i)
	}
	for i := depth - 1; i >= 0; i-- {
		_, _ = fmt.Fprintf(&b, "</e%d>", i)
	}
	return b.String()
}

func TestDeepNesting(t *testing.T) {
	tokens, err := collectTokens(t, nestedDocument(255))
	assert.Nil(t, err)
	assert.Equal(t, 510, len(tokens))
}

func TestNestingDepthLimit(t *testing.T) {
	_, err := collectTokens(t, nestedDocument(256))
	assert.EqualError(t, err, "element nesting depth exceeds 255")
}

func TestMoreThan256Attributes(t *testing.T) {
	var b strings.Builder
	_, _ = b.WriteString("<a")
	for i := 0; i < 300; i++ {
		_, _ = fmt.Fprintf(&b, ` a%d="v"`, i)
	}
	_, _ = b.WriteString("/>")
	dec := gosaxml.NewDecoder(strings.NewReader(b.String()))
	var tk gosaxml.Token
	err := dec.NextToken(&tk)
	assert.Nil(t, err)
	assert.Equal(t, uint8(gosaxml.TokenTypeStartElement), tk.Kind)
	assert.Equal(t, 300, len(tk.Attr))
}

func TestMoreThan26NamespacePrefixes(t *testing.T) {
	var b strings.Builder
	_, _ = b.WriteString("<a")
	for i := 0; i < 30; i++ {
		_, _ = fmt.Fprintf(&b, ` xmlns:p%d="ns%d"`, i, i)
	}
	_, _ = b.WriteString("/>")
	out, err := roundTrip(t, b.String())
	assert.Nil(t, err)
	assert.Contains(t, out, `xmlns:z="ns25"`)
	assert.Contains(t, out, `xmlns:aa="ns26"`)
	assert.Contains(t, out, `xmlns:ad="ns29"`)
}

func TestProcInstWithoutSpace(t *testing.T) {
	tokens, err := collectTokens(t, "<?pi?><a/>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"pi:pi", "start:a", "end:a"}, tokens)
}

func TestXmlSpacePreserveInheritedByChild(t *testing.T) {
	tokens, err := collectTokens(t, "<a xml:space=\"preserve\"><b>\n \n</b></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "start:b", "text:\n \n", "end:b", "end:a"}, tokens)
}

func TestXmlSpacePreserveInheritedByGrandchild(t *testing.T) {
	tokens, err := collectTokens(t, "<a xml:space=\"preserve\"><b><c>\n</c></b></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "start:b", "start:c", "text:\n", "end:c", "end:b", "end:a"}, tokens)
}

func TestXmlSpaceDefaultOverridesPreserve(t *testing.T) {
	tokens, err := collectTokens(t, "<a xml:space=\"preserve\"><b xml:space=\"default\">\n</b></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "start:b", "end:b", "end:a"}, tokens)
}

func TestXmlSpacePreserveNotLeakedToSibling(t *testing.T) {
	tokens, err := collectTokens(t, "<r><a xml:space=\"preserve\">x</a><b>\n \n</b></r>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:r", "start:a", "text:x", "end:a", "start:b", "end:b", "end:r"}, tokens)
}

func TestXmlSpacePreserveClearedByReset(t *testing.T) {
	dec := gosaxml.NewDecoder(strings.NewReader("<a xml:space=\"preserve\"> x </a>"))
	var tk gosaxml.Token
	for {
		if err := dec.NextToken(&tk); err != nil {
			break
		}
	}
	dec.Reset(strings.NewReader("<a>\n \n</a>"))
	var texts []string
	for {
		err := dec.NextToken(&tk)
		if err != nil {
			break
		}
		if tk.Kind == gosaxml.TokenTypeTextElement {
			texts = append(texts, string(tk.ByteData))
		}
	}
	assert.Empty(t, texts)
}

func TestInputOffsetLargerThanReadBuffer(t *testing.T) {
	var b strings.Builder
	_, _ = b.WriteString("<a>")
	for i := 0; i < 300; i++ {
		_, _ = b.WriteString("<x>0123456789</x>")
	}
	_, _ = b.WriteString("</a>")
	input := b.String()
	dec := gosaxml.NewDecoder(strings.NewReader(input))
	var tk gosaxml.Token
	lastOffset := 0
	for {
		err := dec.NextToken(&tk)
		if err == io.EOF {
			break
		}
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, dec.InputOffset(), lastOffset)
		lastOffset = dec.InputOffset()
	}
	assert.Equal(t, len(input), dec.InputOffset())
}

func TestRedeclaredDifferentDefaultNamespaceIsKept(t *testing.T) {
	out, err := roundTrip(t, `<a xmlns="ns1"><b xmlns="ns2"/></a>`)
	assert.Nil(t, err)
	assert.Equal(t, `<a xmlns="ns1"><b xmlns="ns2"/></a>`, out)
}

func TestUnprefixedAttributeStaysUnprefixed(t *testing.T) {
	out, err := roundTrip(t, `<a xmlns:p="ns1"><b xmlns="ns1" c="1"/></a>`)
	assert.Nil(t, err)
	assert.Equal(t, `<a xmlns:a="ns1"><a:b c="1"/></a>`, out)
}

func TestUnmatchedEndElement(t *testing.T) {
	_, err := collectTokens(t, "</a>")
	assert.EqualError(t, err, "unexpected end element without matching start element")
}

func TestRetryAfterErrorDoesNotPanic(t *testing.T) {
	dec := gosaxml.NewDecoder(strings.NewReader("<?"))
	var tk gosaxml.Token
	err := dec.NextToken(&tk)
	assert.Equal(t, io.EOF, err)
	assert.NotPanics(t, func() {
		_ = dec.NextToken(&tk)
	})
}

func TestCDataSectionIsDecodedAsCharData(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[x < y & z]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:x < y & z", "end:a"}, tokens)
}

func TestCDataSectionMayContainMarkup(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[<b>not an element</b>]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:<b>not an element</b>", "end:a"}, tokens)
}

func TestCDataSectionMayContainTwoBrackets(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[a]]b]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:a]]b", "end:a"}, tokens)
}

func TestCDataSectionMayEndWithABracket(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[a]]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:a]", "end:a"}, tokens)
}

func TestEmptyCDataSection(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:", "end:a"}, tokens)
}

func TestCDataSectionKeepsWhitespaceOnlyContent(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[   ]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:   ", "end:a"}, tokens)
}

func TestCDataSectionLargerThanTheReadBuffer(t *testing.T) {
	content := strings.Repeat("abc]]", 10000)
	tokens, err := collectTokens(t, "<a><![CDATA["+content+"]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:" + content, "end:a"}, tokens)
}

func TestUnterminatedCDataSectionIsAnError(t *testing.T) {
	_, err := collectTokens(t, "<a><![CDATA[never closed</a>")
	assert.EqualError(t, err, "invalid XML: unterminated CDATA section")
}

func TestSectionOtherThanCDataIsAnError(t *testing.T) {
	_, err := collectTokens(t, "<a><![FOO[bar]]></a>")
	assert.EqualError(t, err, "invalid XML: expected CDATA section")
}

func TestTextAroundACDataSection(t *testing.T) {
	tokens, err := collectTokens(t, "<a>before<![CDATA[middle]]>after</a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "text:before", "chardata:middle", "text:after", "end:a"}, tokens)
}

func TestTwoCDataSectionsInARow(t *testing.T) {
	tokens, err := collectTokens(t, "<a><![CDATA[one]]><![CDATA[two]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:a", "chardata:one", "chardata:two", "end:a"}, tokens)
}

func TestCDataSectionSurvivesARoundTrip(t *testing.T) {
	out, err := roundTrip(t, "<a><![CDATA[x < y & z]]></a>")
	assert.Nil(t, err)
	assert.Equal(t, "<a><![CDATA[x < y & z]]></a>", out)
}

func TestEncodingCDataSplitsAnEmbeddedDelimiter(t *testing.T) {
	// A CDATA section cannot contain "]]>". A token carrying one must be
	// written as two sections, or the output is not well-formed.
	var w bytes.Buffer
	enc := gosaxml.NewEncoder(&w)
	err := enc.EncodeToken(&gosaxml.Token{
		Kind:     gosaxml.TokenTypeCharData,
		ByteData: []byte("a]]>b"),
	})
	assert.Nil(t, err)
	assert.Nil(t, enc.Flush())
	assert.Equal(t, "<![CDATA[a]]]]><![CDATA[>b]]>", w.String())

	// and the result must decode back to the original text
	tokens, err := collectTokens(t, "<r>"+w.String()+"</r>")
	assert.Nil(t, err)
	assert.Equal(t, []string{"start:r", "chardata:a]]", "chardata:>b", "end:r"}, tokens)
}
