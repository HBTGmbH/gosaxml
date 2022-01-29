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

// EncoderMiddleware allows to pre-process a Token before
// it is finally encoded/written.
type EncoderMiddleware interface {
	EncodeToken(token *Token) error
	Reset()
}

type Encoder struct {
	// the io.Writer we encode/write into
	w io.Writer

	// whether the last token was of type TokenTypeStartElement
	// this is used to delay encoding the ending ">" or "/>" string
	// based on whether the element is immediately closed afterwards.
	lastStartElement bool

	// middlewares can modify encoded tokens before encoding
	middlewares []EncoderMiddleware
}

// NewEncoder creates a new encoder with the given middlewares.
func NewEncoder(w io.Writer, middlewares ...EncoderMiddleware) *Encoder {
	return &Encoder{
		w:           w,
		middlewares: middlewares,
	}
}

func (thiz *Encoder) Reset(w io.Writer) {
	thiz.w = w
	thiz.lastStartElement = false
	for _, middleware := range thiz.middlewares {
		middleware.Reset()
	}
}

func (thiz *Encoder) EncodeToken(t Token) error {
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
	default:
		thiz.lastStartElement = false
		return errors.New("NYI")
	}
	return nil
}

func (thiz *Encoder) encodeStartElement(t Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(bs("<"))
	if err != nil {
		return err
	}

	err = thiz.callMiddlewares(&t)
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
		_, err = thiz.w.Write(bs(" "))
		if err != nil {
			return err
		}
		// attribute name
		err = thiz.writeName(attr.Name)
		if err != nil {
			return err
		}
		// attribute value
		_, err = thiz.w.Write(bs("="))
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

func (thiz *Encoder) encodeEndElement(t Token) error {
	if thiz.lastStartElement {
		// the last seen token was a StartElement, so this
		// token can only be its accompanying EndElement.
		// short-cut it.
		_, err := thiz.w.Write(bs("/>"))
		if err != nil {
			return err
		}
		return thiz.callMiddlewares(&t)
	}

	err := thiz.callMiddlewares(&t)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(bs("</"))
	if err != nil {
		return err
	}
	// write element name
	err = thiz.writeName(t.Name)
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(bs(">"))
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
		_, err = thiz.w.Write(bs(":"))
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

func (thiz Encoder) writeString(s []byte, singleQuote bool) error {
	var err error
	if singleQuote {
		_, err = thiz.w.Write(bs("'"))
	} else {
		_, err = thiz.w.Write(bs("\""))
	}
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(s)
	if err != nil {
		return err
	}
	if singleQuote {
		_, err = thiz.w.Write(bs("'"))
	} else {
		_, err = thiz.w.Write(bs("\""))
	}
	return nil
}

func (thiz *Encoder) encodeTextElement(t Token) error {
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
		_, err := thiz.w.Write(bs(">"))
		if err != nil {
			return err
		}
	}
	return nil
}

func (thiz *Encoder) encodeDirective(t Token) error {
	err := thiz.endLastStartElement()
	if err != nil {
		return err
	}
	_, err = thiz.w.Write(t.ByteData)
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
