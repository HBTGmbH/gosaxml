package gosaxml

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

type Decoder interface {
	NextToken() (Token, error)
	Reset(r io.Reader)
}

type decoder struct {
	lastByte      byte
	hasLastByte   bool
	buf           []byte
	r             *bufio.Reader
	lastOpen      Name
	bb            []byte
	bbOffset      []int
	attrs         []Attr
	numAttributes []int
	top           int
}

// NewDecoder creates a new Decoder.
func NewDecoder(r io.Reader) Decoder {
	return &decoder{
		buf:           make([]byte, 1, 1),
		r:             bufio.NewReader(r),
		bb:            make([]byte, 0, 256),
		bbOffset:      make([]int, 256),
		attrs:         make([]Attr, 0, 256),
		numAttributes: make([]int, 256),
	}
}

func (thiz *decoder) Reset(r io.Reader) {
	thiz.r.Reset(r)
	thiz.attrs = thiz.attrs[:0]
	thiz.bb = thiz.bb[:0]
	thiz.top = 0
}

func (thiz *decoder) skipWhitespaces() error {
	for {
		b, err := thiz.r.ReadByte()
		if err != nil {
			return err
		}
		if b != ' ' && b != '\t' && b != '\b' && b != '\r' && b != '\n' {
			err = thiz.r.UnreadByte()
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func (thiz *decoder) NextToken() (Token, error) {
	var err error
	var b byte
	for {
		// read next character
		b, err = thiz.r.ReadByte()
		if err != nil {
			return Token{}, err
		}
		switch b {
		case '>':
			// Previously StartElement now got properly ended.
			// That's fine. We just did not consume the end token
			// because there could have been an implicit
			// "/>" close at the end of the start element.
		case '/':
			// Immediately closing last openend StartElement.
			// This will generate an EndElement with the same
			// name that we used in the previous StartElement.
			_, err = thiz.r.Discard(1)
			if err != nil {
				return Token{}, err
			}
			return thiz.decodeEndElement(thiz.lastOpen, nil)
		case '<':
			b, err = thiz.r.ReadByte()
			if err != nil {
				return Token{}, err
			}
			switch b {
			case '?':
				return thiz.decodeProcInst()
			case '!':
				// CDATA or comment
				b, err = thiz.r.ReadByte()
				if err != nil {
					return Token{}, err
				}
				switch b {
				case '-':
					err = thiz.ignoreComment()
					if err != nil {
						return Token{}, err
					}
				case '[':
					return thiz.readCDATA()
				default:
					return Token{}, errors.New("invalid XML: comment or CDATA expected")
				}
			case '/':
				return thiz.decodeEndElement(thiz.readName())
			default:
				err = thiz.r.UnreadByte()
				if err != nil {
					return Token{}, err
				}
				return thiz.decodeStartElement()
			}
		default:
			err = thiz.r.UnreadByte()
			if err != nil {
				return Token{}, err
			}
			return thiz.decodeText()
		}
	}
}

func (decoder) decodeProcInst() (Token, error) {
	return Token{}, errors.New("NYI")
}

func (thiz decoder) ignoreComment() error {
	_, err := thiz.r.Discard(1)
	if err != nil {
		return err
	}
	// read until end of comment
	for {
		b, err := thiz.r.ReadByte()
		if err != nil {
			return err
		}
		if b == '-' {
			b2, err := thiz.r.ReadByte()
			if err != nil {
				return err
			}
			if b2 == '-' {
				b3, err := thiz.r.ReadByte()
				if err != nil {
					return err
				}
				if b3 == '>' {
					return nil
				}
			}
		}
	}
}

func (thiz *decoder) decodeEndElement(name Name, err error) (Token, error) {
	if err != nil {
		return Token{}, err
	}
	thiz.top--
	end := len(thiz.attrs) - thiz.numAttributes[thiz.top]
	thiz.attrs = thiz.attrs[0:end]
	thiz.bb = thiz.bb[:thiz.bbOffset[thiz.top]]
	return Token{
		Kind: TokenTypeEndElement,
		Name: name,
	}, nil
}

func (thiz *decoder) decodeStartElement() (Token, error) {
	thiz.numAttributes[thiz.top] = 0
	if thiz.top > 0 {
		thiz.bbOffset[thiz.top] = len(thiz.bb)
	}
	name, err := thiz.readName()
	if err != nil {
		return Token{}, err
	}
	attributes, err := thiz.readAttributes()
	if err != nil {
		return Token{}, err
	}
	thiz.lastOpen = name
	thiz.top++
	return Token{
		Kind: TokenTypeStartElement,
		Name: name,
		Attr: attributes,
	}, nil
}

func (thiz *decoder) decodeText() (Token, error) {
	i := len(thiz.bb)
	for {
		b, err := thiz.r.ReadByte()
		if err != nil {
			return Token{}, err
		}
		switch b {
		case '<':
			err = thiz.r.UnreadByte()
			if err != nil {
				return Token{}, err
			}
			return Token{
				Kind:     TokenTypeTextElement,
				ByteData: thiz.bb[i:len(thiz.bb)],
			}, nil
		default:
			thiz.bb = append(thiz.bb, b)
		}
	}
}

func (thiz decoder) readCDATA() (Token, error) {
	// discard "CDATA["
	_, err := thiz.r.Discard(6)
	if err != nil {
		return Token{}, err
	}
	return Token{}, errors.New("NYI")
}

func (thiz *decoder) readName() (Name, error) {
	for {
		localOrPrefix, err := thiz.readSimpleName()
		if err != nil {
			return Name{}, err
		}
		b, err := thiz.r.ReadByte()
		if err != nil {
			return Name{}, err
		}
		switch b {
		case ':':
			local, err := thiz.readSimpleName()
			if err != nil {
				return Name{}, err
			}
			return Name{
				Local:  local,
				Prefix: localOrPrefix,
			}, nil
		case ' ', '/', '=', '>':
			err = thiz.r.UnreadByte()
			if err != nil {
				return Name{}, err
			}
			return Name{
				Local: localOrPrefix,
			}, nil
		default:
			return Name{}, errors.New("reached here unexpectedly")
		}
	}
}

func (thiz *decoder) readSimpleName() ([]byte, error) {
	i := len(thiz.bb)
	for {
		b, err := thiz.r.ReadByte()
		if err != nil {
			return nil, err
		}
		switch b {
		case ':', '>', ' ', '/', '=':
			err = thiz.r.UnreadByte()
			if err != nil {
				return nil, err
			}
			return thiz.bb[i:len(thiz.bb)], nil
		default:
			thiz.bb = append(thiz.bb, b)
		}
	}
}

func (thiz *decoder) readAttributes() ([]Attr, error) {
	i := len(thiz.attrs)
	for {
		err := thiz.skipWhitespaces()
		if err != nil {
			return nil, err
		}
		b, err := thiz.r.ReadByte()
		if err != nil {
			return nil, nil
		}
		switch b {
		case '/', '>':
			err = thiz.r.UnreadByte()
			if err != nil {
				return nil, err
			}
			return thiz.attrs[i:len(thiz.attrs)], nil
		default:
			err = thiz.r.UnreadByte()
			if err != nil {
				return nil, err
			}
			i := len(thiz.attrs)
			thiz.attrs = thiz.attrs[:i+1]
			err := thiz.readAttribute(&thiz.attrs[i])
			if err != nil {
				return nil, err
			}
			thiz.numAttributes[thiz.top]++
		}
	}
}

// readAttribute parses a single XML attribute
// after this function returns, the next reader symbol
// is the byte after the closing single or double quote
// of the attribute's value.
func (thiz *decoder) readAttribute(attr *Attr) error {
	name, err := thiz.readName()
	if err != nil {
		return err
	}
	err = thiz.skipWhitespaces()
	if err != nil {
		return err
	}
	b, err := thiz.r.ReadByte()
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
	attr.Name = name
	attr.SingleQuote = singleQuote
	attr.Value = value
	return nil
}

// readString parses a single string (in single or double quotes)
func (thiz *decoder) readString() ([]byte, bool, error) {
	b, err := thiz.r.ReadByte()
	if err != nil {
		return nil, false, err
	}
	i := len(thiz.bb)
	j := i
	singleQuote := b == '\''
	for {
		b, err := thiz.r.ReadByte()
		if err != nil {
			return nil, false, err
		}
		switch b {
		case '"':
			if !singleQuote {
				if err != nil {
					return nil, false, err
				}
				return thiz.bb[i:j], singleQuote, nil
			}
			thiz.bb = append(thiz.bb, b)
			j++
		case '\'':
			if singleQuote {
				if err != nil {
					return nil, false, err
				}
				return thiz.bb[i:j], singleQuote, nil
			}
			thiz.bb = append(thiz.bb, b)
			j++
		default:
			thiz.bb = append(thiz.bb, b)
			j++
		}
	}
}
