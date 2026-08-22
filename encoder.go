package gosaxml

import (
	"errors"
	"io"
)

// pre-allocate all constant byte slices that we write
var (
	// all characters used to build new namespace aliases
	namespaceAliases = []byte("abcdefghijklmnopqrstuvwxyz")

	// constant strings needed here and there
	angleOpenQuest  = []byte("<?")
	questAngleClose = []byte("?>")
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

// encodeBufferSize is the initial size of the Encoder's output buffer.
const encodeBufferSize = 8192

// encodeBlock is the block size used by appendBlock. Every reservation asks
// for one block of headroom on top of the bytes it actually needs, so that
// the block copies do not fall back to memmove at the end of the buffer.
const encodeBlock = 32

// Encoder encodes Token values to an io.Writer.
type Encoder struct {
	// buffers writes to the underlying io.Writer
	buf []byte

	// middlewares can modify encoded tokens before encoding.
	middlewares []EncoderMiddleware

	// The io.Writer we encode/write into.
	wr io.Writer

	// Whether the last token was of type TokenTypeStartElement.
	// This is used to delay encoding the ending ">" or "/>" string
	// based on whether the element is immediately closed afterwards.
	lastStartElement bool
}

// NewEncoder creates a new Encoder with the given middlewares and returns a pointer to it.
func NewEncoder(w io.Writer, middlewares ...EncoderMiddleware) *Encoder {
	return &Encoder{
		buf:         make([]byte, 0, encodeBufferSize),
		wr:          w,
		middlewares: middlewares,
	}
}

// Flush writes all buffered output into the io.Writer.
// It must be called after token encoding is done in order
// to write all remaining bytes into the io.Writer.
func (thiz *Encoder) Flush() error {
	if len(thiz.buf) == 0 {
		return nil
	}
	_, err := thiz.wr.Write(thiz.buf)
	thiz.buf = thiz.buf[:0]
	return err
}

// reserve makes sure that at least n more bytes can be appended to the
// output buffer without it having to grow, flushing (and, if a single token
// is larger than the whole buffer, growing) as needed.
// Reserving up-front lets the actual encoding be a straight run of appends
// with no per-write flush checks.
func (thiz *Encoder) reserve(n int) error {
	if len(thiz.buf)+n+encodeBlock <= cap(thiz.buf) {
		return nil
	}
	return thiz.reserveSlow(n)
}

func (thiz *Encoder) reserveSlow(n int) error {
	err := thiz.Flush()
	if err != nil {
		return err
	}
	n += encodeBlock
	if n > cap(thiz.buf) {
		c := cap(thiz.buf) * 2
		for c < n {
			c *= 2
		}
		thiz.buf = make([]byte, 0, c)
	}
	return nil
}

// Reset resets this Encoder to write into the provided io.Writer
// and resets all middlewares.
func (thiz *Encoder) Reset(w io.Writer) {
	thiz.wr = w
	thiz.buf = thiz.buf[:0]
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
		err := thiz.encodeBytes(t.ByteData)
		if err != nil {
			return err
		}
		thiz.lastStartElement = false
	case TokenTypeDirective:
		err := thiz.encodeBytes(t.ByteData)
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
	case TokenTypeInvalid:
		return errors.New("trying to encode invalid/zerovalue token")
	default:
		thiz.lastStartElement = false
		return errors.New("NYI")
	}
	return nil
}

// appendBlock appends src to dst with a fixed-size block copy that the
// compiler expands inline, and reports whether it could. It declines when the
// block would not fit into either slice, in which case the caller falls back
// to a plain append.
//
// XML names and attribute values are short, and for those this saves the
// runtime.memmove call that a variable-length copy compiles to. Reading a
// whole block out of src is safe because src is required to have the capacity
// for it; the surplus bytes are cut away again by re-slicing.
func appendBlock(dst, src []byte) ([]byte, bool) {
	l := len(dst)
	if len(src) > encodeBlock || l+encodeBlock > cap(dst) || cap(src) < encodeBlock {
		return dst, false
	}
	d := dst[:l+encodeBlock]
	*(*[encodeBlock]byte)(d[l:]) = *(*[encodeBlock]byte)(src[:encodeBlock])
	return d[:l+len(src)], true
}

// appendVar appends src to dst, using a block copy where possible.
func appendVar(dst, src []byte) []byte {
	if d, ok := appendBlock(dst, src); ok {
		return d
	}
	return append(dst, src...)
}

// nameLen returns the number of bytes that appendName will write for n.
// A non-nil (even if empty) prefix is written, followed by ':'.
func nameLen(n Name) int {
	l := len(n.Local)
	if n.Prefix != nil {
		l += len(n.Prefix) + 1
	}
	return l
}

// appendName appends the (possibly prefixed) name to buf.
// The caller must have reserved nameLen(n) bytes.
//
// The hot encoding paths spell this out instead of calling it, so that
// appendVar — and with it the block copy — inlines into them.
func appendName(buf []byte, n Name) []byte {
	if n.Prefix != nil {
		buf = appendVar(buf, n.Prefix)
		buf = append(buf, ':')
	}
	return appendVar(buf, n.Local)
}

func (thiz *Encoder) encodeStartElement(t *Token) error {
	// Middlewares only rewrite the token, they never write output, so they
	// can run before we size and emit the element.
	err := thiz.callMiddlewares(t)
	if err != nil {
		return err
	}

	// ">" of the pending start element + "<" + name
	n := 2 + nameLen(t.Name)
	attrs := t.Attr
	for i := 0; i < len(attrs); i++ {
		attr := &attrs[i]
		// ' ' + name + '=' + quote + value + quote
		n += 4 + nameLen(attr.Name) + len(attr.Value)
	}
	err = thiz.reserve(n)
	if err != nil {
		return err
	}

	buf := thiz.buf
	if thiz.lastStartElement {
		buf = append(buf, '>')
	}
	buf = append(buf, '<')
	if t.Name.Prefix != nil {
		buf = appendVar(buf, t.Name.Prefix)
		buf = append(buf, ':')
	}
	buf = appendVar(buf, t.Name.Local)
	for i := 0; i < len(attrs); i++ {
		attr := &attrs[i]
		buf = append(buf, ' ')
		if attr.Name.Prefix != nil {
			buf = appendVar(buf, attr.Name.Prefix)
			buf = append(buf, ':')
		}
		buf = appendVar(buf, attr.Name.Local)
		q := byte('"')
		if attr.SingleQuote {
			q = '\''
		}
		buf = append(buf, '=', q)
		buf = appendVar(buf, attr.Value)
		buf = append(buf, q)
	}
	thiz.buf = buf

	// DO NOT write the ending ">" character, because the element
	// could get closed right away with the next EndElement token.

	return nil
}

func (thiz *Encoder) encodeEndElement(t *Token) error {
	if thiz.lastStartElement {
		// the last seen token was a StartElement, so this
		// token can only be its accompanying EndElement.
		err := thiz.reserve(2)
		if err != nil {
			return err
		}
		thiz.buf = append(thiz.buf, '/', '>')
		return thiz.callMiddlewares(t)
	}

	err := thiz.callMiddlewares(t)
	if err != nil {
		return err
	}
	err = thiz.reserve(3 + nameLen(t.Name))
	if err != nil {
		return err
	}
	buf := append(thiz.buf, '<', '/')
	if t.Name.Prefix != nil {
		buf = appendVar(buf, t.Name.Prefix)
		buf = append(buf, ':')
	}
	buf = appendVar(buf, t.Name.Local)
	thiz.buf = append(buf, '>')
	return nil
}

func (thiz *Encoder) callMiddlewares(t *Token) error {
	for _, middleware := range thiz.middlewares {
		err := middleware.EncodeToken(t)
		if err != nil {
			return err
		}
	}
	return nil
}

// encodeBytes ends a pending start element and then writes bs verbatim.
func (thiz *Encoder) encodeBytes(bs []byte) error {
	err := thiz.reserve(len(bs) + 1)
	if err != nil {
		return err
	}
	buf := thiz.buf
	if thiz.lastStartElement {
		buf = append(buf, '>')
	}
	thiz.buf = appendVar(buf, bs)
	return nil
}

func (thiz *Encoder) encodeProcInst(t *Token) error {
	err := thiz.reserve(6 + nameLen(t.Name) + len(t.ByteData))
	if err != nil {
		return err
	}
	buf := thiz.buf
	if thiz.lastStartElement {
		buf = append(buf, '>')
	}
	buf = append(buf, angleOpenQuest...)
	buf = appendName(buf, t.Name)
	buf = append(buf, ' ')
	buf = append(buf, t.ByteData...)
	thiz.buf = append(buf, questAngleClose...)
	return nil
}
