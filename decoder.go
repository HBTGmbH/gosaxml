package gosaxml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

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
	bbOffset            [256]int32
	numAttributes       [256]byte
	lastOpen            Name
	preserveWhitespaces [32]bool
	r                   bufreader
	bb                  []byte
	attrs               []Attr
	buf                 [8]byte
	read                byte
	write               byte
	top                 byte
}

// NewDecoder creates a new Decoder.
func NewDecoder(r io.Reader) Decoder {
	return &decoder{
		r: bufreader{
			rd: r,
		},
		bb:    make([]byte, 0, 256),
		attrs: make([]Attr, 0, 256),
	}
}

func isWhitespace(b byte) bool {
	return b == '\t' || b == '\n' || b == '\r' || b == ' '
}

func (thiz *decoder) Reset(r io.Reader) {
	thiz.r.reset(r)
	thiz.attrs = thiz.attrs[:0]
	thiz.bb = thiz.bb[:0]
	thiz.top = 0
}

func (thiz *decoder) skipWhitespaces(b byte) (byte, error) {
	for {
		if !isWhitespace(b) {
			return b, nil
		}
		var err error
		b, err = thiz.r.readByte()
		if err != nil {
			return 0, err
		}
	}
}

func (thiz *decoder) NextToken(t *Token) error {
	for {
		// read next character
		b, err := thiz.r.readByte()
		if err != nil {
			return err
		}
		switch b {
		case '>':
			// Previous StartElement now got properly ended.
			// That's fine. We just did not consume the end token
			// because there could have been an implicit
			// "/>" close at the end of the start element.
		case '/':
			// Immediately closing last openend StartElement.
			// This will generate an EndElement with the same
			// name that we used in the previous StartElement.
			_, err = thiz.r.discard(1)
			if err != nil {
				return err
			}
			return thiz.decodeEndElement(t, thiz.lastOpen)
		case '<':
			b, err = thiz.r.readByte()
			if err != nil {
				return err
			}
			switch b {
			case '?':
				err = thiz.decodeProcInst(t)
				thiz.r.unreadByte()
				return err
			case '!':
				// CDATA or comment
				b, err = thiz.r.readByte()
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
				return thiz.decodeEndElement(t, name)
			default:
				return thiz.decodeStartElement(t)
			}
		default:
			thiz.r.unreadByte()
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
				b2, err = thiz.r.readByte()
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
		b, err = thiz.r.readByte()
		if err != nil {
			return err
		}
	}
}

func (thiz *decoder) ignoreComment() error {
	_, err := thiz.r.discard(1)
	if err != nil {
		return err
	}
	for {
		var b byte
		b, err = thiz.r.readByte()
		if err != nil {
			return err
		}
		if b == '-' {
			var b2 byte
			b2, err = thiz.r.readByte()
			if err != nil {
				return err
			}
			if b2 == '-' {
				for {
					var b3 byte
					b3, err = thiz.r.readByte()
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
	thiz.r.unreadByte()
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
	thiz.r.unreadByte()
	return nil
}

func (thiz *decoder) decodeText(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		b, err := thiz.r.readByte()
		if err != nil {
			return false, err
		}
		if b == '<' {
			thiz.r.unreadByte()
			if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
				return true, nil
			}
			t.Kind = TokenTypeTextElement
			t.ByteData = thiz.bb[i:len(thiz.bb)]
			return false, nil
		}
		onlyWhitespaces = onlyWhitespaces && isWhitespace(b)
		thiz.bb = append(thiz.bb, b)
	}
}

func (thiz *decoder) readCDATA() error {
	// discard "CDATA["
	_, err := thiz.r.discard(6)
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
		j := thiz.r.r
		for k := j; k < thiz.r.w; k++ {
			if isSeparator(thiz.r.buf[k]) {
				thiz.bb = append(thiz.bb, thiz.r.buf[j:k]...)
				_, err := thiz.r.discard(k - j + 1)
				if err != nil {
					return nil, 0, err
				}
				return thiz.bb[i:len(thiz.bb)], thiz.r.buf[k], nil
			}
		}
		thiz.bb = append(thiz.bb, thiz.r.buf[thiz.r.r:thiz.r.w]...)
		thiz.r.discardBuffer()
		err := thiz.r.read0()
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
			b, err = thiz.r.readByte()
			thiz.numAttributes[thiz.top]++
		}
	}
}

// decodeAttribute parses a single XML attribute.
// After this function returns, the next reader symbol
// is the byte after the closing single or double quote
// of the attribute's value.
func (thiz *decoder) decodeAttribute(attr *Attr) error {
	thiz.r.unreadByte()
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
	b, err = thiz.r.readByte()
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
	if bytes.Equal(name.Prefix, bs("xml")) && bytes.Equal(name.Local, bs("space")) {
		thiz.preserveWhitespaces[thiz.top] = bytes.Equal(value, bs("preserve"))
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
		j := thiz.r.r
		k := bytes.IndexByte(thiz.r.buf[j:thiz.r.w], b)
		if k > -1 {
			thiz.bb = append(thiz.bb, thiz.r.buf[j:j+k]...)
			_, err := thiz.r.discard(k + 1)
			if err != nil {
				return nil, false, err
			}
			return thiz.bb[i:len(thiz.bb)], singleQuote, nil
		}
		thiz.bb = append(thiz.bb, thiz.r.buf[thiz.r.r:thiz.r.w]...)
		thiz.r.discardBuffer()
		err := thiz.r.read0()
		if err != nil {
			return nil, false, err
		}
	}
}
