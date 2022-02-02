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

func (thiz *decoder) skipWhitespaces() error {
	for {
		b, err := thiz.r.readByte()
		if err != nil {
			return err
		}
		if !isWhitespace(b) {
			thiz.r.unreadByte()
			return nil
		}
	}
}

func (thiz *decoder) NextToken(t *Token) error {
	var err error
	var b byte
	for {
		// read next character
		b, err = thiz.r.readByte()
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
				return thiz.decodeProcInst(t)
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
				name, err := thiz.readName()
				if err != nil {
					return err
				}
				return thiz.decodeEndElement(t, name)
			default:
				thiz.r.unreadByte()
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
	name, err := thiz.readName()
	if err != nil {
		return err
	}
	err = thiz.skipWhitespaces()
	if err != nil {
		return err
	}
	i := len(thiz.bb)
	j := i
	for {
		b, err := thiz.r.readByte()
		if err != nil {
			return err
		}
		if b == '?' {
			for {
				b2, err := thiz.r.readByte()
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
	}
}

func (thiz *decoder) ignoreComment() error {
	_, err := thiz.r.discard(1)
	if err != nil {
		return err
	}
	for {
		b, err := thiz.r.readByte()
		if err != nil {
			return err
		}
		if b == '-' {
			b2, err := thiz.r.readByte()
			if err != nil {
				return err
			}
			if b2 == '-' {
				for {
					b3, err := thiz.r.readByte()
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
	name, err := thiz.readName()
	if err != nil {
		return err
	}
	attributes, err := thiz.decodeAttributes()
	if err != nil {
		return err
	}
	thiz.lastOpen = name
	t.Kind = TokenTypeStartElement
	t.Name = name
	t.Attr = attributes
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

func (thiz *decoder) readName() (Name, error) {
	localOrPrefix, err := thiz.readSimpleName()
	if err != nil {
		return Name{}, err
	}
	b, err := thiz.r.readByte()
	if err != nil {
		return Name{}, err
	}
	if b == ':' {
		local, err := thiz.readSimpleName()
		if err != nil {
			return Name{}, err
		}
		return Name{
			Local:  local,
			Prefix: localOrPrefix,
		}, nil
	} else if isSeparator(b) {
		thiz.r.unreadByte()
		return Name{
			Local: localOrPrefix,
		}, nil
	} else {
		return Name{}, errors.New("reached here unexpectedly")
	}
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

func (thiz *decoder) readSimpleName() ([]byte, error) {
	i := len(thiz.bb)
	for {
		b, err := thiz.r.readByte()
		if err != nil {
			return nil, err
		}
		if isSeparator(b) {
			thiz.r.unreadByte()
			return thiz.bb[i:len(thiz.bb)], nil
		}
		thiz.bb = append(thiz.bb, b)
	}
}

func (thiz *decoder) decodeAttributes() ([]Attr, error) {
	i := len(thiz.attrs)
	for {
		err := thiz.skipWhitespaces()
		if err != nil {
			return nil, err
		}
		b, err := thiz.r.readByte()
		if err != nil {
			return nil, nil
		}
		switch b {
		case '/', '>':
			thiz.r.unreadByte()
			return thiz.attrs[i:len(thiz.attrs)], nil
		default:
			thiz.r.unreadByte()
			i := len(thiz.attrs)
			thiz.attrs = thiz.attrs[:i+1]
			err := thiz.decodeAttribute(&thiz.attrs[i])
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
	name, err := thiz.readName()
	if err != nil {
		return err
	}
	err = thiz.skipWhitespaces()
	if err != nil {
		return err
	}
	b, err := thiz.r.readByte()
	if err != nil {
		return err
	}
	if b != '=' {
		return fmt.Errorf("expected '=' character following attribute %+v", name)
	}
	err = thiz.skipWhitespaces()
	if err != nil {
		return err
	}
	value, singleQuote, err := thiz.readString()
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
func (thiz *decoder) readString() ([]byte, bool, error) {
	b, err := thiz.r.readByte()
	if err != nil {
		return nil, false, err
	}
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
		err = thiz.r.read0()
		if err != nil {
			return nil, false, err
		}
	}
}
