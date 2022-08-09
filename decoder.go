package gosaxml

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/klauspost/cpuid/v2"
	"io"
)

var canUseSSE = cpuid.CPU.Has(cpuid.SSE) && cpuid.CPU.Has(cpuid.BMI1)

// Decoder decodes an XML input stream into Token values.
type Decoder interface {
	// NextToken decodes and stores the next Token into
	// the provided Token pointer.
	// Only the fields relevant for the decoded token type
	// are written to the Token. Other fields may have previous
	// values. The caller should thus determine the Token.Kind
	// and then only read/touch the fields relevant for that kind.
	NextToken(t *Token) error

	// Reset resets the Decoder to the given io.Reader.
	Reset(r io.Reader)
}

type decoder struct {
	rb                  [2048]byte
	bbOffset            [256]int32
	numAttributes       [256]byte
	lastOpen            Name
	preserveWhitespaces [32]bool
	rd                  io.Reader
	bb                  []byte
	attrs               []Attr
	r                   int
	w                   int
	top                 byte
	lastStartElement    bool
}

var (
	bsxml      = []byte("xml")
	bsspace    = []byte("space")
	bspreserve = []byte("preserve")
)

// NewDecoder creates a new Decoder.
func NewDecoder(r io.Reader) Decoder {
	return &decoder{
		rd:    r,
		bb:    make([]byte, 0, 256),
		attrs: make([]Attr, 0, 256),
	}
}

func isWhitespace(b byte) bool {
	return b <= ' '
}

func (thiz *decoder) read0() error {
	if thiz.r > 0 {
		copy(thiz.rb[:], thiz.rb[thiz.r:thiz.w])
		thiz.w -= thiz.r
		thiz.r = 0
	}
	n, err := thiz.rd.Read(thiz.rb[thiz.w : cap(thiz.rb)-16])
	thiz.w += n
	if n <= 0 && err != nil {
		return err
	}
	return nil
}

func (thiz *decoder) unreadByte() {
	thiz.r--
}

func (thiz *decoder) readByte() (byte, error) {
	for thiz.r == thiz.w {
		err := thiz.read0()
		if err != nil {
			return 0, err
		}
	}
	c := thiz.rb[thiz.r]
	thiz.r++
	return c, nil
}

func (thiz *decoder) discardBuffer() {
	thiz.r = thiz.w
}

func (thiz *decoder) discard(n int) (int, error) {
	for thiz.r+n > thiz.w {
		err := thiz.read0()
		if err != nil {
			return 0, err
		}
	}
	thiz.r += n
	return n, nil
}

func (thiz *decoder) Reset(r io.Reader) {
	thiz.rd = r
	thiz.r = 0
	thiz.w = 0
	thiz.attrs = thiz.attrs[:0]
	thiz.bb = thiz.bb[:0]
	thiz.top = 0
	thiz.lastStartElement = false
}

func (thiz *decoder) skipWhitespaces(b byte) (byte, error) {
	for {
		if !isWhitespace(b) {
			return b, nil
		}
		var err error
		b, err = thiz.readByte()
		if err != nil {
			return 0, err
		}
	}
}

func (thiz *decoder) NextToken(t *Token) error {
	for {
		// read next character
		b, err := thiz.readByte()
		if err != nil {
			return err
		}
		switch b {
		case '>':
			// Previous StartElement now got properly ended.
			// That's fine. We just did not consume the end token
			// because there could have been an implicit
			// "/>" close at the end of the start element.
			thiz.lastStartElement = false
		case '/':
			if thiz.lastStartElement {
				// Immediately closing last openend StartElement.
				// This will generate an EndElement with the same
				// name that we used in the previous StartElement.
				_, err = thiz.discard(1)
				if err != nil {
					return err
				}
				thiz.lastStartElement = false
				return thiz.decodeEndElement(t, thiz.lastOpen)
			}
			thiz.unreadByte()
			cntn, err := thiz.decodeText(t)
			if err != nil || !cntn {
				return err
			}
		case '<':
			b, err = thiz.readByte()
			if err != nil {
				return err
			}
			switch b {
			case '?':
				thiz.lastStartElement = false
				err = thiz.decodeProcInst(t)
				thiz.unreadByte()
				return err
			case '!':
				// CDATA or comment
				b, err = thiz.readByte()
				if err != nil {
					return err
				}
				switch b {
				case '-':
					err = thiz.ignoreComment()
					if err != nil {
						return err
					}
				case '[':
					thiz.lastStartElement = false
					return thiz.readCDATA()
				default:
					return errors.New("invalid XML: comment or CDATA expected")
				}
			case '/':
				var name Name
				name, _, err = thiz.readName()
				if err != nil {
					return err
				}
				thiz.lastStartElement = false
				return thiz.decodeEndElement(t, name)
			default:
				thiz.lastStartElement = true
				return thiz.decodeStartElement(t)
			}
		default:
			thiz.lastStartElement = false
			thiz.unreadByte()
			cntn, err := thiz.decodeText(t)
			if err != nil || !cntn {
				return err
			}
		}
	}
}

func (thiz *decoder) decodeProcInst(t *Token) error {
	name, b, err := thiz.readName()
	if err != nil {
		return err
	}
	b, err = thiz.skipWhitespaces(b)
	if err != nil {
		return err
	}
	i := len(thiz.bb)
	j := i
	for {
		if b == '?' {
			for {
				var b2 byte
				b2, err = thiz.readByte()
				if err != nil {
					return err
				}
				if b2 == '>' {
					t.Kind = TokenTypeProcInst
					t.Name = name
					t.ByteData = thiz.bb[i:j]
					return nil
				} else if b2 != '?' {
					thiz.bb = append(thiz.bb, b, b2)
					if !isWhitespace(b2) {
						j = len(thiz.bb)
					}
					break
				}
				thiz.bb = append(thiz.bb, b2)
				if !isWhitespace(b2) {
					j = len(thiz.bb)
				}
			}
		} else {
			thiz.bb = append(thiz.bb, b)
			if !isWhitespace(b) {
				j = len(thiz.bb)
			}
		}
		b, err = thiz.readByte()
		if err != nil {
			return err
		}
	}
}

func (thiz *decoder) ignoreComment() error {
	_, err := thiz.discard(1)
	if err != nil {
		return err
	}
	for {
		var b byte
		b, err = thiz.readByte()
		if err != nil {
			return err
		}
		if b == '-' {
			var b2 byte
			b2, err = thiz.readByte()
			if err != nil {
				return err
			}
			if b2 == '-' {
				for {
					var b3 byte
					b3, err = thiz.readByte()
					if err != nil {
						return err
					}
					if b3 == '>' {
						return nil
					} else if b3 != '-' {
						break
					}
				}
			}
		}
	}
}

func (thiz *decoder) decodeEndElement(t *Token, name Name) error {
	end := len(thiz.attrs) - int(thiz.numAttributes[thiz.top])
	thiz.attrs = thiz.attrs[0:end]
	thiz.bb = thiz.bb[:thiz.bbOffset[thiz.top]]
	t.Kind = TokenTypeEndElement
	t.Name = name
	thiz.top--
	return nil
}

func (thiz *decoder) decodeStartElement(t *Token) error {
	thiz.top++
	thiz.numAttributes[thiz.top] = 0
	thiz.bbOffset[thiz.top] = int32(len(thiz.bb))
	thiz.preserveWhitespaces[thiz.top+1] = thiz.preserveWhitespaces[thiz.top]
	thiz.unreadByte()
	name, b, err := thiz.readName()
	if err != nil {
		return err
	}
	var attributes []Attr
	attributes, err = thiz.decodeAttributes(b)
	if err != nil {
		return err
	}
	thiz.lastOpen = name
	t.Kind = TokenTypeStartElement
	t.Name = name
	t.Attr = attributes
	thiz.unreadByte()
	return nil
}

func (thiz *decoder) decodeTextSSE(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		j := thiz.r
		c := 0
		for thiz.w > thiz.r+c {
			sidx := findFirstOpenAngleBracket16(thiz.rb[j+c : thiz.w])
			onlyWhitespaces = onlyWhitespaces && onlySpacesUntil16(thiz.rb[j+c:thiz.w], sidx)
			c += sidx
			if sidx != 16 {
				_, err := thiz.discard(c)
				if err != nil {
					return false, err
				}
				if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
					return true, nil
				}
				thiz.bb = append(thiz.bb, thiz.rb[j:j+c]...)
				t.Kind = TokenTypeTextElement
				t.ByteData = thiz.bb[i:len(thiz.bb)]
				return false, nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return false, err
		}
	}
}

func (thiz *decoder) decodeText(t *Token) (bool, error) {
	if canUseSSE {
		return thiz.decodeTextSSE(t)
	}
	return thiz.decodeTextGeneric(t)
}

func (thiz *decoder) decodeTextGeneric(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		j := thiz.r
		for k := j; k < thiz.w; k++ {
			b := thiz.rb[k]
			if b == '<' {
				_, err := thiz.discard(k - j)
				if err != nil {
					return false, err
				}
				if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
					return true, nil
				}
				thiz.bb = append(thiz.bb, thiz.rb[j:k]...)
				t.Kind = TokenTypeTextElement
				t.ByteData = thiz.bb[i:len(thiz.bb)]
				return false, nil
			}
			onlyWhitespaces = onlyWhitespaces && isWhitespace(b)
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return false, err
		}
	}
}

func (thiz *decoder) readCDATA() error {
	// discard "CDATA["
	_, err := thiz.discard(6)
	if err != nil {
		return err
	}
	return errors.New("NYI")
}

func (thiz *decoder) readName() (Name, byte, error) {
	localOrPrefix, b, err := thiz.readSimpleName()
	if err != nil {
		return Name{}, 0, err
	}
	if b == ':' {
		var local []byte
		local, b, err = thiz.readSimpleName()
		if err != nil {
			return Name{}, 0, err
		}
		return Name{
			Local:  local,
			Prefix: localOrPrefix,
		}, b, nil
	}
	return Name{
		Local: localOrPrefix,
	}, b, nil
}

var seps = generateTable()

func generateTable() ['>' + 1]bool {
	var s ['>' + 1]bool
	s['\t'] = true
	s['\n'] = true
	s['\r'] = true
	s[' '] = true
	s['/'] = true
	s[':'] = true
	s['='] = true
	s['>'] = true
	return s
}

func isSeparator(b byte) bool {
	return int(b) < len(seps) && seps[b]
}

func (thiz *decoder) readSimpleName() ([]byte, byte, error) {
	i := len(thiz.bb)
	for {
		j := thiz.r
		for k := j; k < thiz.w; k++ {
			if isSeparator(thiz.rb[k]) {
				thiz.bb = append(thiz.bb, thiz.rb[j:k]...)
				_, err := thiz.discard(k - j + 1)
				if err != nil {
					return nil, 0, err
				}
				return thiz.bb[i:len(thiz.bb)], thiz.rb[k], nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return nil, 0, err
		}
	}
}

func (thiz *decoder) decodeAttributes(b byte) ([]Attr, error) {
	i := len(thiz.attrs)
	for {
		var err error
		b, err = thiz.skipWhitespaces(b)
		if err != nil {
			return nil, err
		}
		switch b {
		case '/', '>':
			return thiz.attrs[i:len(thiz.attrs)], nil
		default:
			i := len(thiz.attrs)
			thiz.attrs = thiz.attrs[:i+1]
			err = thiz.decodeAttribute(&thiz.attrs[i])
			if err != nil {
				return nil, err
			}
			b, err = thiz.readByte()
			if err != nil {
				return nil, err
			}
			thiz.numAttributes[thiz.top]++
		}
	}
}

// decodeAttribute parses a single XML attribute.
// After this function returns, the next reader symbol
// is the byte after the closing single or double quote
// of the attribute's value.
func (thiz *decoder) decodeAttribute(attr *Attr) error {
	thiz.unreadByte()
	name, b, err := thiz.readName()
	if err != nil {
		return err
	}
	b, err = thiz.skipWhitespaces(b)
	if err != nil {
		return err
	}
	if b != '=' {
		return fmt.Errorf("expected '=' character following attribute %+v", name)
	}
	b, err = thiz.readByte()
	if err != nil {
		return err
	}
	b, err = thiz.skipWhitespaces(b)
	if err != nil {
		return err
	}
	value, singleQuote, err := thiz.readString(b)
	if err != nil {
		return err
	}
	// xml:space?
	if bytes.Equal(name.Prefix, bsxml) && bytes.Equal(name.Local, bsspace) {
		thiz.preserveWhitespaces[thiz.top] = bytes.Equal(value, bspreserve)
	}
	attr.Name = name
	attr.SingleQuote = singleQuote
	attr.Value = value
	return nil
}

// readString parses a single string (in single or double quotes)
func (thiz *decoder) readString(b byte) ([]byte, bool, error) {
	i := len(thiz.bb)
	singleQuote := b == '\''
	for {
		j := thiz.r
		k := bytes.IndexByte(thiz.rb[j:thiz.w], b)
		if k > -1 {
			thiz.bb = append(thiz.bb, thiz.rb[j:j+k]...)
			_, err := thiz.discard(k + 1)
			if err != nil {
				return nil, false, err
			}
			return thiz.bb[i:len(thiz.bb)], singleQuote, nil
		}
		thiz.bb = append(thiz.bb, thiz.rb[j:thiz.w]...)
		thiz.discardBuffer()
		err := thiz.read0()
		if err != nil {
			return nil, false, err
		}
	}
}
