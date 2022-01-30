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
	angleOpen       = bs("<")
	angleClose      = bs(">")
	slashAngleClose = bs("/>")
	angleOpenSlash  = bs("</")
	space           = bs(" ")
	equal           = bs("=")
	angleOpenQuest  = bs("<?")
	questAngleClose = bs("?>")
	colon           = bs(":")
	singleQuote     = bs("'")
	doubleQuote     = bs("\"")
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
	// The io.Writer we encode/write into.
	w io.Writer

	// Whether the last token was of type TokenTypeStartElement.
	// This is used to delay encoding the ending ">" or "/>" string
	// based on whether the element is immediately closed afterwards.
	lastStartElement bool

	// middlewares can modify encoded tokens before encoding.
	middlewares []EncoderMiddleware
}

// NewEncoder creates a new Encoder with the given middlewares and returns a pointer to it.
func NewEncoder(w io.Writer, middlewares ...EncoderMiddleware) *Encoder {
	return &Encoder{
		w:           w,
		middlewares: middlewares,
	}
}

// Reset resets this Encoder to write into the provided io.Writer
// and resets all middlewares.
func (thiz *Encoder) Reset(w io.Writer) {
	thiz.w = w
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
	_, err = thiz.w.Write(angleOpen)
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
	for _, attr := range t.Attr {
		_, err = thiz.w.Write(space)
		if err != nil {
			return err
		}
		err = thiz.writeName(attr.Name)
		if err != nil {
			return err
		}
		_, err = thiz.w.Write(equal)
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
		_, err := thiz.w.Write(slashAngleClose)
		if err != nil {
			return err
		}
		return thiz.callMiddlewares(t)
	}

	err := thiz.callMiddlewares(t)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(angleOpenSlash)
	if err != nil {
		return err
	}
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(angleClose)
	if err != nil {
		return err
	}
	return nil
}

func (thiz *Encoder) callMiddlewares(t *Token) error {
	var err error
	for _, middleware := range thiz.middlewares {
		err = middleware.EncodeToken((*Token)(noescape(unsafe.Pointer(t))))
		if err != nil {
			return err
		}
	}
	return nil
}

func (thiz Encoder) writeName(n Name) error {
	var err error
	if n.Prefix != nil {
		_, err = thiz.w.Write(n.Prefix)
		if err != nil {
			return err
		}
		_, err = thiz.w.Write(colon)
		if err != nil {
			return err
		}
	}
	_, err = thiz.w.Write(n.Local)
	if err != nil {
		return err
	}
	return nil
}

func (thiz Encoder) writeString(s []byte, useSingleQuote bool) error {
	var err error
	if useSingleQuote {
		_, err = thiz.w.Write(singleQuote)
	} else {
		_, err = thiz.w.Write(doubleQuote)
	}
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(s)
	if err != nil {
		return err
	}
	if useSingleQuote {
		_, err = thiz.w.Write(singleQuote)
	} else {
		_, err = thiz.w.Write(doubleQuote)
	}
	return nil
}

func (thiz *Encoder) encodeTextElement(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(t.ByteData)
	return err
}

func (thiz *Encoder) endLastStartElement() error {
	if thiz.lastStartElement {
		// end the last StartElement with its ">"
		_, err := thiz.w.Write(angleClose)
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
	_, err = thiz.w.Write(t.ByteData)
	return err
}

func (thiz *Encoder) encodeProcInst(t *Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(angleOpenQuest)
	if err != nil {
		return err
	}
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(space)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(t.ByteData)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(questAngleClose)
	return err
}

// https://stackoverflow.com/questions/59209493/how-to-use-unsafe-get-a-byte-slice-from-a-string-without-memory-copy#answer-59210739
func bs(s string) []byte {
	if s == "" {
		return []byte{}
	}
	return (*[0x7fff0000]byte)(unsafe.Pointer(
		(*reflect.StringHeader)(unsafe.Pointer(&s)).Data),
	)[:len(s):len(s)]
}

// https://go.googlesource.com/go/+/go1.17.6/src/runtime/stubs.go#164
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}
