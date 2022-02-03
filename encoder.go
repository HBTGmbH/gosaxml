package gosaxml

import (
	"errors"
	"io"
	"reflect"
	"unsafe"
)

const (
	// all characters used to build new namespace aliases
	namespaceAliases = "abcdefghijklmnopqrstuvwxyz"
)

// pre-allocate all constant byte slices that we write
var (
	slashAngleClose = bs("/>")
	angleOpenSlash  = bs("</")
	angleOpenQuest  = bs("<?")
	questAngleClose = bs("?>")
)

// EncoderMiddleware allows to pre-process a Token before
// it is finally encoded/written.
type EncoderMiddleware interface {
	// EncodeToken will be called by the Encoder before the provided Token
	// is finally byte-encoded into the io.Writer.
	// The Encoder will ensure that the pointed-to Token and all its contained
	// field values will remain unmodified for the lexical scope of the
	// XML-element represented by the Token.
	// If, for example, the Token represents a TokenTypeStartElement, then
	// the Token and all of its contained fields/byte-slices will contain
	// their values until after its corresponding TokenTypeEndElement is processed
	// by the EncoderMiddleware.
	EncodeToken(token *Token) error

	// Reset resets the state of an EncoderMiddleware.
	// This can be used to e.g. reset all pre-allocated data structures
	// and reinitialize the EncoderMiddleware to the state before the
	// any first call to EncodeToken.
	Reset()
}

// Encoder encodes Token values to an io.Writer.
type Encoder struct {
	// buffers writes to the underlying io.Writer
	buf [2048]byte

	// middlewares can modify encoded tokens before encoding.
	middlewares []EncoderMiddleware

	// The io.Writer we encode/write into.
	wr io.Writer

	// the current write position into buf
	w int

	// Whether the last token was of type TokenTypeStartElement.
	// This is used to delay encoding the ending ">" or "/>" string
	// based on whether the element is immediately closed afterwards.
	lastStartElement bool
}

// NewEncoder creates a new Encoder with the given middlewares and returns a pointer to it.
func NewEncoder(w io.Writer, middlewares ...EncoderMiddleware) *Encoder {
	return &Encoder{
		wr:          w,
		middlewares: middlewares,
	}
}

// Flush writes all buffered output into the io.Writer.
// It must be called after token encoding is done in order
// to write all remaining bytes into the io.Writer.
func (thiz *Encoder) Flush() error {
	_, err := thiz.wr.Write(thiz.buf[:thiz.w])
	thiz.w = 0
	return err
}

func (thiz *Encoder) write(b byte) error {
	if thiz.w >= len(thiz.buf) {
		err := thiz.Flush()
		if err != nil {
			return err
		}
	}
	thiz.buf[thiz.w] = b
	thiz.w++
	return nil
}

func (thiz *Encoder) writeBytes(bs []byte) error {
	l := len(bs)
	if thiz.w+l > len(thiz.buf) {
		err := thiz.Flush()
		if err != nil {
			return err
		}
	}
	copy(thiz.buf[thiz.w:], bs)
	thiz.w += l
	return nil
}

// Reset resets this Encoder to write into the provided io.Writer
// and resets all middlewares.
func (thiz *Encoder) Reset(w io.Writer) {
	thiz.wr = w
	thiz.w = 0
	thiz.lastStartElement = false
	for _, middleware := range thiz.middlewares {
		middleware.Reset()
	}
}

// EncodeToken first calls any EncoderMiddleware and then
// writes the byte-representation of that Token to the io.Writer
// of this Encoder.
func (thiz *Encoder) EncodeToken(t *Token) error {
	switch t.Kind {
	case TokenTypeStartElement:
		err := thiz.encodeStartElement(t)
		if err != nil {
			return err
		}
		thiz.lastStartElement = true
	case TokenTypeEndElement:
		err := thiz.encodeEndElement(t)
		if err != nil {
			return err
		}
		thiz.lastStartElement = false
	case TokenTypeTextElement:
		err := thiz.encodeTextElement(t)
		if err != nil {
			return err
		}
		thiz.lastStartElement = false
	case TokenTypeDirective:
		err := thiz.encodeDirective(t)
		if err != nil {
			return err
		}
		thiz.lastStartElement = false
	case TokenTypeProcInst:
		err := thiz.encodeProcInst(t)
		if err != nil {
			return err
		}
		thiz.lastStartElement = false
	default:
		thiz.lastStartElement = false
		return errors.New("NYI")
	}
	return nil
}

func (thiz *Encoder) encodeStartElement(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	err = thiz.write('<')
	if err != nil {
		return err
	}

	err = thiz.callMiddlewares(t)
	if err != nil {
		return err
	}

	// write element name
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}

	// write attributes
	for i := 0; i < len(t.Attr); i++ {
		attr := &t.Attr[i]
		err = thiz.write(' ')
		if err != nil {
			return err
		}
		err = thiz.writeName(attr.Name)
		if err != nil {
			return err
		}
		err = thiz.write('=')
		if err != nil {
			return err
		}
		err = thiz.writeString(attr.Value, attr.SingleQuote)
		if err != nil {
			return err
		}
	}

	// DO NOT write the ending ">" character, because the element
	// could get closed right away with the next EndElement token.

	return nil
}

func (thiz *Encoder) encodeEndElement(t *Token) error {
	if thiz.lastStartElement {
		// the last seen token was a StartElement, so this
		// token can only be its accompanying EndElement.
		err := thiz.writeBytes(slashAngleClose)
		if err != nil {
			return err
		}
		return thiz.callMiddlewares(t)
	}

	err := thiz.callMiddlewares(t)
	if err != nil {
		return err
	}
	err = thiz.writeBytes(angleOpenSlash)
	if err != nil {
		return err
	}
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}
	err = thiz.write('>')
	if err != nil {
		return err
	}
	return nil
}

func (thiz *Encoder) callMiddlewares(t *Token) error {
	var err error
	for _, middleware := range thiz.middlewares {
		err = middleware.EncodeToken(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (thiz *Encoder) writeName(n Name) error {
	var err error
	if n.Prefix != nil {
		err = thiz.writeBytes(n.Prefix)
		if err != nil {
			return err
		}
		err = thiz.write(':')
		if err != nil {
			return err
		}
	}
	return thiz.writeBytes(n.Local)
}

func (thiz *Encoder) writeString(s []byte, useSingleQuote bool) error {
	var err error
	if useSingleQuote {
		err = thiz.write('\'')
	} else {
		err = thiz.write('"')
	}
	if err != nil {
		return err
	}
	err = thiz.writeBytes(s)
	if err != nil {
		return err
	}
	if useSingleQuote {
		err = thiz.write('\'')
	} else {
		err = thiz.write('"')
	}
	return err
}

func (thiz *Encoder) encodeTextElement(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	return thiz.writeBytes(t.ByteData)
}

func (thiz *Encoder) endLastStartElement() error {
	if thiz.lastStartElement {
		// end the last StartElement with its ">"
		err := thiz.write('>')
		if err != nil {
			return err
		}
	}
	return nil
}

func (thiz *Encoder) encodeDirective(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	return thiz.writeBytes(t.ByteData)
}

func (thiz *Encoder) encodeProcInst(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	err = thiz.writeBytes(angleOpenQuest)
	if err != nil {
		return err
	}
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}
	err = thiz.write(' ')
	if err != nil {
		return err
	}
	err = thiz.writeBytes(t.ByteData)
	if err != nil {
		return err
	}
	err = thiz.writeBytes(questAngleClose)
	return err
}

// https://stackoverflow.com/questions/59209493/how-to-use-unsafe-get-a-byte-slice-from-a-string-without-memory-copy#answer-59210739
func bs(s string) []byte {
	return (*[0x7fff0000]byte)(unsafe.Pointer(
		(*reflect.StringHeader)(unsafe.Pointer(&s)).Data),
	)[:len(s):len(s)]
}
