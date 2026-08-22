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

	// InputOffset returns the current offset in the input stream.
	InputOffset() int

	// Reset resets the Decoder to the given io.Reader.
	Reset(r io.Reader)
}

// readBufferSize is the size of the decoder's input window.
// Larger windows amortize the cost of io.Reader.Read calls and of
// buffer compaction over more input bytes.
const readBufferSize = 8192

// overcopy is the fixed block size used by appendFast. The read buffer is
// padded by this much so that a fixed-size (and therefore compiler-inlined)
// copy may always read a whole block, even for the very last name in the
// window.
const overcopy = 32

type decoder struct {
	// The input window. Only rb[:readBufferSize] ever holds input; the
	// trailing overcopy bytes exist purely as read-ahead padding.
	rb                  [readBufferSize + overcopy]byte
	bbOffset            [256]int32
	numAttributes       [256]int32
	lastOpen            Name
	preserveWhitespaces [256]bool
	rd                  io.Reader
	bb                  []byte
	attrs               []Attr
	r                   int
	w                   int
	off                 int
	top                 byte
	// Correction applied by InputOffset. decodeAttributes consumes the "/"
	// or ">" that ends a start element right away, but InputOffset has
	// always reported the position of that byte, so it is backed out again
	// until the next token is decoded. Kept as an addend rather than a flag
	// so that neither NextToken nor InputOffset needs a branch for it.
	offAdjust int
	// Set when the start element just returned ended in "/" and therefore
	// still owes the caller its EndElement.
	pendingSelfClose bool
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
		bb:    make([]byte, 0, 1024),
		attrs: make([]Attr, 0, 256),
	}
}

func isWhitespace(b byte) bool {
	return b <= ' '
}

// appendFast copies src, which must be a slice of the current input window,
// onto the token buffer and reports whether it could.
//
// Names and attribute values are short, and for those a fixed-size block move
// that the compiler expands inline is markedly cheaper than the
// runtime.memmove call a variable-length copy compiles to. Reading a whole
// block out of the input window is safe because that window is padded by
// overcopy bytes; the surplus bytes are cut away again by re-slicing.
//
// It declines for sources longer than a block, and when the token buffer has
// no room for a whole block — appendFromInputSlow handles both. It is
// call-free so that it inlines into the scanning loops.
func (thiz *decoder) appendFast(src []byte) bool {
	l := len(thiz.bb)
	if len(src) > overcopy || l+overcopy > cap(thiz.bb) {
		return false
	}
	bb := thiz.bb[:l+overcopy]
	*(*[overcopy]byte)(bb[l:]) = *(*[overcopy]byte)(src[:overcopy])
	thiz.bb = bb[:l+len(src)]
	return true
}

func (thiz *decoder) appendFromInputSlow(src []byte) {
	if len(src) > overcopy {
		thiz.bb = append(thiz.bb, src...)
		return
	}
	if len(thiz.bb)+overcopy > cap(thiz.bb) {
		thiz.growBB(len(thiz.bb) + overcopy)
	}
	if !thiz.appendFast(src) {
		thiz.bb = append(thiz.bb, src...)
	}
}

// growBB grows the token buffer so that it can hold at least n bytes.
// Already-returned token slices keep pointing into the old backing array,
// whose contents stay valid, just as they do when append grows the buffer.
func (thiz *decoder) growBB(n int) {
	c := cap(thiz.bb) * 2
	for c < n {
		c *= 2
	}
	nb := make([]byte, len(thiz.bb), c)
	copy(nb, thiz.bb)
	thiz.bb = nb
}

func (thiz *decoder) read0() error {
	if thiz.r > 0 {
		copy(thiz.rb[:], thiz.rb[thiz.r:thiz.w])
		thiz.off += thiz.r
		thiz.w -= thiz.r
		thiz.r = 0
	}
	n, err := thiz.rd.Read(thiz.rb[thiz.w:readBufferSize])
	thiz.w += n
	if n <= 0 && err != nil {
		return err
	}
	return nil
}

func (thiz *decoder) unreadByte() {
	thiz.r--
}

// readByteFast returns the next input byte, or false if the input window is
// exhausted and the caller has to go through readByteSlow. It is call-free so
// that it inlines into the token loop.
func (thiz *decoder) readByteFast() (byte, bool) {
	if thiz.r < thiz.w {
		b := thiz.rb[thiz.r]
		thiz.r++
		return b, true
	}
	return 0, false
}

func (thiz *decoder) readByte() (byte, error) {
	if thiz.r < thiz.w {
		c := thiz.rb[thiz.r]
		thiz.r++
		return c, nil
	}
	return thiz.readByteSlow()
}

func (thiz *decoder) readByteSlow() (byte, error) {
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

func (thiz *decoder) InputOffset() int {
	return thiz.off + thiz.r + thiz.offAdjust
}

func (thiz *decoder) Reset(r io.Reader) {
	thiz.rd = r
	thiz.r = 0
	thiz.w = 0
	thiz.off = 0
	thiz.attrs = thiz.attrs[:0]
	thiz.bb = thiz.bb[:0]
	thiz.top = 0
	thiz.lastOpen = Name{}
	thiz.preserveWhitespaces = [256]bool{}
	thiz.offAdjust = 0
	thiz.pendingSelfClose = false
}

func (thiz *decoder) NextToken(t *Token) error {
	thiz.offAdjust = 0
	if thiz.pendingSelfClose {
		// The previous StartElement was closed immediately ("/>"), so its
		// EndElement — carrying the same name — is due now. The ">" is only
		// consumed here, so that a truncated input still fails on this call
		// rather than on the one that produced the StartElement.
		thiz.pendingSelfClose = false
		_, err := thiz.discard(1)
		if err != nil {
			return err
		}
		t.Name = thiz.lastOpen
		return thiz.decodeEndElement(t)
	}
	for {
		// read next character
		b, ok := thiz.readByteFast()
		var err error
		if !ok {
			b, err = thiz.readByteSlow()
			if err != nil {
				return err
			}
		}
		switch b {
		case '<':
			b, ok = thiz.readByteFast()
			if !ok {
				b, err = thiz.readByteSlow()
				if err != nil {
					return err
				}
			}
			switch b {
			case '?':
				return thiz.decodeProcInst(t)
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
					return thiz.readCDATA()
				default:
					return errors.New("invalid XML: comment or CDATA expected")
				}
			case '/':
				_, err = thiz.readName(&t.Name)
				if err != nil {
					return err
				}
				return thiz.decodeEndElement(t)
			default:
				return thiz.decodeStartElement(t)
			}
		case '>':
			// A ">" that does not close any markup we opened. Skip it.
		default:
			thiz.unreadByte()
			cntn, err := thiz.decodeText(t)
			if err != nil || !cntn {
				return err
			}
		}
	}
}

func (thiz *decoder) decodeProcInst(t *Token) error {
	b, err := thiz.readName(&t.Name)
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
		for thiz.w > thiz.r {
			k := bytes.IndexByte(thiz.rb[thiz.r:thiz.w], '-')
			if k > -1 {
				_, err = thiz.discard(k + 1)
				if err != nil {
					return err
				}
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
			} else {
				thiz.discardBuffer()
			}
		}
		err := thiz.read0()
		if err != nil {
			return err
		}
	}
}

// decodeEndElement finishes an end element whose name is already in t.Name.
func (thiz *decoder) decodeEndElement(t *Token) error {
	if thiz.top == 0 {
		return errors.New("unexpected end element without matching start element")
	}
	end := len(thiz.attrs) - int(thiz.numAttributes[thiz.top])
	thiz.attrs = thiz.attrs[0:end]
	thiz.bb = thiz.bb[:thiz.bbOffset[thiz.top]]
	t.Kind = TokenTypeEndElement
	thiz.top--
	return nil
}

func (thiz *decoder) decodeStartElement(t *Token) error {
	if thiz.top == 255 {
		return errors.New("element nesting depth exceeds 255")
	}
	thiz.top++
	thiz.numAttributes[thiz.top] = 0
	thiz.bbOffset[thiz.top] = int32(len(thiz.bb))
	// inherit xml:space handling from the parent element (may be
	// overridden by an xml:space attribute in decodeAttribute)
	thiz.preserveWhitespaces[thiz.top] = thiz.preserveWhitespaces[thiz.top-1]
	thiz.unreadByte()
	b, err := thiz.readName(&t.Name)
	if err != nil {
		return err
	}
	err = thiz.decodeAttributes(b, t)
	if err != nil {
		return err
	}
	if thiz.pendingSelfClose {
		// Only an immediately closed element needs its name remembered for
		// the EndElement that NextToken still owes; copying a Name is 48
		// bytes, so it is not worth doing for every element.
		thiz.lastOpen = t.Name
	}
	t.Kind = TokenTypeStartElement
	return nil
}

// decodeText scans character data up to (but not including) the next '<'.
//
// It returns true when the scanned run consisted of whitespace only and
// whitespace is not being preserved, in which case no token is produced and
// the caller simply continues scanning.
//
// The common cases — a run of ignorable indentation, or a text node that lies
// entirely within the current input window — are handled here; everything
// else goes to decodeTextSlow.
func (thiz *decoder) decodeText(t *Token) (bool, error) {
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	// A single scan locates the first byte that is neither whitespace nor
	// '<' ('<' is > ' ', so it terminates the scan as well). That answers
	// both questions at once: where the text ends and whether it was all
	// whitespace.
	p := indexNonSpaceHead(buf)
	if p < 0 {
		if len(buf) <= scalarPrefix {
			return thiz.decodeTextSlow(t)
		}
		// long run of indentation: let the vectorized scanner find its end
		p = indexNonSpaceLong(buf)
		if p < 0 {
			return thiz.decodeTextSlow(t)
		}
	}
	if buf[p] == '<' {
		if thiz.preserveWhitespaces[thiz.top] {
			return thiz.decodeTextSlow(t)
		}
		thiz.r = j + p
		return true, nil
	}
	k := indexOfAngle(buf[p:])
	if k < 0 {
		return thiz.decodeTextSlow(t)
	}
	k += p
	i := len(thiz.bb)
	if !thiz.appendFast(buf[:k]) {
		thiz.appendFromInputSlow(buf[:k])
	}
	thiz.r = j + k
	t.Kind = TokenTypeTextElement
	t.ByteData = thiz.bb[i:]
	return false, nil
}

// decodeTextSlow handles character data that spans more than one fill of the
// input window, as well as whitespace runs longer than the scalar prefix.
func (thiz *decoder) decodeTextSlow(t *Token) (bool, error) {
	i := len(thiz.bb)
	onlyWhitespaces := true
	for {
		j := thiz.r
		buf := thiz.rb[j:thiz.w]
		var k int
		if onlyWhitespaces {
			p := indexNonSpaceHead(buf)
			if p < 0 && len(buf) > scalarPrefix {
				p = indexNonSpaceLong(buf)
			}
			switch {
			case p < 0:
				k = -1
			case buf[p] == '<':
				k = p
			default:
				onlyWhitespaces = false
				k = indexOfAngle(buf[p:])
				if k >= 0 {
					k += p
				}
			}
		} else {
			k = indexOfAngle(buf)
		}
		if k >= 0 {
			thiz.r = j + k
			if onlyWhitespaces && !thiz.preserveWhitespaces[thiz.top] {
				return true, nil
			}
			if !thiz.appendFast(buf[:k]) {
				thiz.appendFromInputSlow(buf[:k])
			}
			t.Kind = TokenTypeTextElement
			t.ByteData = thiz.bb[i:]
			return false, nil
		}
		thiz.bb = append(thiz.bb, buf...)
		thiz.r = thiz.w
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

// readName reads an element or attribute name, with or without a prefix, and
// returns it together with the separator byte that terminated it, which is
// consumed.
//
// The whole name is scanned and copied here when it lies inside the current
// input window, which saves a call layer per name over going through
// readSimpleName — names are the most frequent thing a decoder reads.
// readName reads an element or attribute name, with or without a prefix,
// stores it in n and returns the separator byte that terminated it, which is
// consumed. Both fields of n are always assigned.
//
// The whole name is scanned and copied here when it lies inside the current
// input window, which saves a call layer per name over going through
// readSimpleName — names are the most frequent thing a decoder reads. Writing
// through a pointer keeps the 48-byte Name out of the return values, where it
// would have to be spilled by every caller.
func (thiz *decoder) readName(n *Name) (byte, error) {
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	k := indexSeparatorHead(buf)
	if k >= 0 {
		i := len(thiz.bb)
		if buf[k] != ':' {
			if thiz.appendFast(buf[:k]) {
				thiz.r = j + k + 1
				n.Local = thiz.bb[i:]
				n.Prefix = nil
				return buf[k], nil
			}
		} else {
			rest := buf[k+1:]
			if k2 := indexSeparatorHead(rest); k2 >= 0 && thiz.appendFast(buf[:k]) {
				prefix := thiz.bb[i:]
				i2 := len(thiz.bb)
				if thiz.appendFast(rest[:k2]) {
					thiz.r = j + k + k2 + 2
					n.Prefix = prefix
					n.Local = thiz.bb[i2:]
					return rest[k2], nil
				}
				// undo the prefix; the general path redoes the whole name
				thiz.bb = thiz.bb[:i]
			}
		}
	}
	return thiz.readNameSlow(n, k)
}

// readNameSlow reads a name that readName could not complete. k is the
// separator index its head scan already found in the current input window,
// or -1 if it found none.
func (thiz *decoder) readNameSlow(n *Name, k int) (byte, error) {
	localOrPrefix, b, err := thiz.readSimpleNameSlow(len(thiz.bb), k)
	if err != nil {
		return 0, err
	}
	if b == ':' {
		var local []byte
		local, b, err = thiz.readSimpleName()
		if err != nil {
			return 0, err
		}
		n.Prefix = localOrPrefix
		n.Local = local
		return b, nil
	}
	n.Prefix = nil
	n.Local = localOrPrefix
	return b, nil
}

// readSimpleName reads a single (prefix-less) name and returns it together
// with the separator byte that terminated it, which is consumed.
//
// Everything that is not "the name ends inside the current input window,
// within the scalar prefix, and fits into the token buffer as it stands" is
// pushed into readSimpleNameSlow. Keeping this — by far the hottest function
// of the decoder — free of loops and of all but one call site measurably
// improves the code the compiler generates for it.
func (thiz *decoder) readSimpleName() ([]byte, byte, error) {
	i := len(thiz.bb)
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	k := indexSeparatorHead(buf)
	if k >= 0 && thiz.appendFast(buf[:k]) {
		thiz.r = j + k + 1
		return thiz.bb[i:], buf[k], nil
	}
	return thiz.readSimpleNameSlow(i, k)
}

// readSimpleNameSlow finishes a name that readSimpleName could not complete.
// k is the separator index that the caller's head scan already found in the
// current input window, or -1 if it found none — in which case the head of
// that window does not have to be scanned again.
func (thiz *decoder) readSimpleNameSlow(i, k int) ([]byte, byte, error) {
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	if k < 0 && len(buf) > scalarPrefix {
		k = indexSeparatorLong(buf)
	}
	for {
		if k >= 0 {
			thiz.r = j + k + 1
			if !thiz.appendFast(buf[:k]) {
				thiz.appendFromInputSlow(buf[:k])
			}
			return thiz.bb[i:], buf[k], nil
		}
		thiz.bb = append(thiz.bb, buf...)
		thiz.r = thiz.w
		err := thiz.read0()
		if err != nil {
			return nil, 0, err
		}
		j = thiz.r
		buf = thiz.rb[j:thiz.w]
		k = indexSeparatorHead(buf)
		if k < 0 && len(buf) > scalarPrefix {
			k = indexSeparatorLong(buf)
		}
	}
}

// skipWhitespaces returns b if b is not whitespace, otherwise it consumes
// all following whitespace and returns the first non-whitespace byte.
//
// The overwhelmingly common case is that b already is a non-whitespace byte,
// so that test is kept small enough for this to be inlined into its callers.
func (thiz *decoder) skipWhitespaces(b byte) (byte, error) {
	if b > ' ' {
		return b, nil
	}
	return thiz.skipWhitespacesSlow()
}

func (thiz *decoder) skipWhitespacesSlow() (byte, error) {
	for {
		j := thiz.r
		buf := thiz.rb[j:thiz.w]
		k := indexNonSpaceHead(buf)
		if k < 0 && len(buf) > scalarPrefix {
			k = indexNonSpaceLong(buf)
		}
		if k >= 0 {
			thiz.r = j + k + 1
			return buf[k], nil
		}
		thiz.r = thiz.w
		err := thiz.read0()
		if err != nil {
			return 0, err
		}
	}
}

// decodeAttributes parses the attributes of a start element and stores them
// in t.Attr. b is the byte that terminated the element name.
func (thiz *decoder) decodeAttributes(b byte, t *Token) error {
	i := len(thiz.attrs)
	for {
		if b <= ' ' {
			// Find the next significant byte. Doing the scan here rather
			// than through readByte plus skipWhitespaces keeps the single
			// space that usually separates two attributes call-free.
			buf := thiz.rb[thiz.r:thiz.w]
			p := indexNonSpaceHead(buf)
			if p >= 0 {
				b = buf[p]
				thiz.r += p + 1
			} else {
				var err error
				b, err = thiz.skipWhitespacesSlow()
				if err != nil {
					return err
				}
			}
		}
		if b == '/' || b == '>' {
			// The terminator is consumed here instead of being read and
			// dispatched on again by the next NextToken call. A "/" leaves
			// an EndElement pending; offAdjust keeps InputOffset reporting
			// the terminator's own position in the meantime.
			thiz.offAdjust = -1
			thiz.pendingSelfClose = b == '/'
			t.Attr = thiz.attrs[i:]
			return nil
		}
		// Extend by one without writing a zero Attr first: decodeAttribute
		// assigns every field of it anyway, and an Attr is 80 bytes.
		j := len(thiz.attrs)
		if j < cap(thiz.attrs) {
			thiz.attrs = thiz.attrs[:j+1]
		} else {
			thiz.attrs = append(thiz.attrs, Attr{})
		}
		err := thiz.decodeAttribute(&thiz.attrs[j])
		if err != nil {
			return err
		}
		thiz.numAttributes[thiz.top]++
		// Scan on from the input for the next attribute or the terminator.
		b = ' '
	}
}

// decodeAttribute parses a single XML attribute.
// After this function returns, the next reader symbol
// is the byte after the closing single or double quote
// of the attribute's value.
func (thiz *decoder) decodeAttribute(attr *Attr) error {
	thiz.unreadByte()
	b, err := thiz.readName(&attr.Name)
	if err != nil {
		return err
	}
	if b != '=' {
		b, err = thiz.skipWhitespaces(b)
		if err != nil {
			return err
		}
		if b != '=' {
			return fmt.Errorf("expected '=' character following attribute %+v", attr.Name)
		}
	}

	// The quote almost always follows the '=' directly, and the value almost
	// always ends within the current input window. Handling that here spares
	// a call into readString for every attribute.
	var value []byte
	var singleQuote, haveValue bool
	buf := thiz.rb[thiz.r:thiz.w]
	if len(buf) > 1 && (buf[0] == '"' || buf[0] == '\'') {
		q := buf[0]
		if v := indexByteHead(buf[1:], q, quotePrefix); v >= 0 {
			i := len(thiz.bb)
			if thiz.appendFast(buf[1 : 1+v]) {
				thiz.r += v + 2
				value, singleQuote, haveValue = thiz.bb[i:], q == '\'', true
			}
		}
	}
	if !haveValue {
		b, err = thiz.readByte()
		if err != nil {
			return err
		}
		b, err = thiz.skipWhitespaces(b)
		if err != nil {
			return err
		}
		value, singleQuote, err = thiz.readString(b)
		if err != nil {
			return err
		}
	}

	// xml:space?
	if len(attr.Name.Prefix) == 3 && attr.Name.Prefix[0] == 'x' && len(attr.Name.Local) == 5 &&
		bytes.Equal(attr.Name.Prefix, bsxml) && bytes.Equal(attr.Name.Local, bsspace) {
		thiz.preserveWhitespaces[thiz.top] = bytes.Equal(value, bspreserve)
	}
	attr.SingleQuote = singleQuote
	attr.Value = value
	return nil
}

// readString parses a single string (in single or double quotes).
func (thiz *decoder) readString(b byte) ([]byte, bool, error) {
	i := len(thiz.bb)
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	// Attribute values are usually short; scanning the first few bytes
	// inline avoids the call into bytes.IndexByte for those.
	k := indexByteHead(buf, b, quotePrefix)
	if k >= 0 && thiz.appendFast(buf[:k]) {
		thiz.r = j + k + 1
		return thiz.bb[i:], b == '\'', nil
	}
	return thiz.readStringSlow(i, b, k)
}

// readStringSlow finishes a string that readString could not complete.
// k is the index of the terminating quote that the caller already found in
// the current input window, or -1 if it found none within the scanned head.
func (thiz *decoder) readStringSlow(i int, b byte, k int) ([]byte, bool, error) {
	singleQuote := b == '\''
	j := thiz.r
	buf := thiz.rb[j:thiz.w]
	if k < 0 && len(buf) > quotePrefix {
		k = bytes.IndexByte(buf[quotePrefix:], b)
		if k >= 0 {
			k += quotePrefix
		}
	}
	for {
		if k >= 0 {
			thiz.r = j + k + 1
			if !thiz.appendFast(buf[:k]) {
				thiz.appendFromInputSlow(buf[:k])
			}
			return thiz.bb[i:], singleQuote, nil
		}
		thiz.bb = append(thiz.bb, buf...)
		thiz.r = thiz.w
		err := thiz.read0()
		if err != nil {
			return nil, false, err
		}
		j = thiz.r
		buf = thiz.rb[j:thiz.w]
		k = bytes.IndexByte(buf, b)
	}
}
