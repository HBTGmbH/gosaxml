package gosaxml

import (
	"bytes"
	"errors"
)

// NamespaceModifier can be used to obtain information about the
// effective namespace of a decoded Token via NamespaceOfToken
// and to canonicalize/minify namespace declarations.
type NamespaceModifier struct {
	openNames         [32]Name
	nsOffs            [32]int32
	prefixAliasesOffs [32]int32

	namespaces    [][]byte
	prefixAliases [][]byte

	top byte

	PreserveOriginalPrefixes bool
}

// NewNamespaceModifier creates a new NamespaceModifier and returns a pointer to it.
func NewNamespaceModifier() *NamespaceModifier {
	return &NamespaceModifier{
		namespaces:    make([][]byte, 0, 64),
		prefixAliases: make([][]byte, 0, 64),
	}
}

// Reset resets this NamespaceModifier.
func (thiz *NamespaceModifier) Reset() {
	thiz.top = 0
	thiz.namespaces = thiz.namespaces[:0]
	thiz.prefixAliases = thiz.prefixAliases[:0]
}

// EncodeToken will be called by the Encoder before the provided Token
// is finally byte-encoded into the io.Writer.
// The Encoder will ensure that the pointed-to Token and all its contained
// field values will remain unmodified for the lexical scope of the
// XML-element represented by the Token.
// If, for example, the Token represents a TokenTypeStartElement, then
// the Token and all of its contained fields/byte-slices will contain
// their values until after its corresponding TokenTypeEndElement is processed
// by the EncoderMiddleware.
func (thiz *NamespaceModifier) EncodeToken(t *Token) error {
	if t.Kind == TokenTypeStartElement {
		err := thiz.pushFrame()
		if err != nil {
			return err
		}
		thiz.processNamespaces(t)
		thiz.processElementName(t)
		thiz.openNames[thiz.top] = t.Name
	} else if t.Kind == TokenTypeEndElement {
		thiz.processElementName(t)
		err := thiz.popFrame()
		if err != nil {
			return err
		}
	}
	return nil
}

func (thiz *NamespaceModifier) processElementName(t *Token) {
	if t.Kind == TokenTypeStartElement {
		if len(thiz.prefixAliases) > 0 {
			// check attributes for rewritten prefixes
			for i := 0; i < len(t.Attr); i++ {
				attr := &t.Attr[i]
				prefix := thiz.findPrefixAlias(attr.Name.Prefix)
				if prefix != nil {
					attr.Name.Prefix = prefix
				}
			}
			// Did we rewrite the element name prefix?
			prefix := thiz.findPrefixAlias(t.Name.Prefix)
			if prefix != nil {
				t.Name.Prefix = prefix
			}
		}
	} else if t.Kind == TokenTypeEndElement {
		t.Name = thiz.openNames[thiz.top]
	}
}

// findNamespaceForPrefix finds the namespace bound to the given prefix (if any)
// within the stack frame of the current element
// This is the reverse operation of findPrefixForNamespace.
func (thiz *NamespaceModifier) findNamespaceForPrefix(prefix []byte) []byte {
	// scan all frames up to the top
	for i := len(thiz.namespaces)/2 - 1; i >= 0; i-- {
		if bytes.Equal(thiz.namespaces[2*i], prefix) {
			return thiz.namespaces[2*i+1]
		}
	}
	return nil
}

// findPrefixForNamespace finds the prefix which binds the given namespace (if any)
// This is the reverse operation of findNamespaceForPrefix.
func (thiz *NamespaceModifier) findPrefixForNamespace(namespace []byte) []byte {
	// scan all frames up to the top
	for i := len(thiz.namespaces)/2 - 1; i >= 0; i-- {
		if bytes.Equal(thiz.namespaces[2*i+1], namespace) {
			return thiz.namespaces[2*i]
		}
	}
	return nil
}

// findPrefixAlias finds the alias for the given prefix.
// There is an alias for a given prefix it, during encoding, the prefix
// has been replaced with a (possibly) shorter alternative.
func (thiz *NamespaceModifier) findPrefixAlias(prefix []byte) []byte {
	// scan all frames up to the top
	for i := len(thiz.prefixAliases)/2 - 1; i >= 0; i-- {
		if bytes.Equal(thiz.prefixAliases[2*i], prefix) {
			return thiz.prefixAliases[2*i+1]
		}
	}
	return nil
}

func (thiz *NamespaceModifier) pushFrame() error {
	if thiz.top >= 255 {
		return errors.New("stack overflow")
	}
	thiz.top++
	thiz.nsOffs[thiz.top] = thiz.nsOffs[thiz.top-1]
	thiz.prefixAliasesOffs[thiz.top] = thiz.prefixAliasesOffs[thiz.top-1]
	return nil
}

func (thiz *NamespaceModifier) popFrame() error {
	if thiz.top <= 0 {
		return errors.New("stack underflow")
	}
	thiz.top--
	off := thiz.nsOffs[thiz.top]
	thiz.namespaces = thiz.namespaces[:off*2]
	off = thiz.prefixAliasesOffs[thiz.top]
	thiz.prefixAliases = thiz.prefixAliases[:off*2]
	return nil
}

// processNamespaces scans the attributes of the given token for namespace declarations,
// either with or without a binding prefix and possibly re-assigns prefixes to other existing
// or new aliases and drops redundant namespace declarations.
func (thiz *NamespaceModifier) processNamespaces(t *Token) {
	j := 0
	for i := 0; i < len(t.Attr); i++ {
		attr := &t.Attr[i]
		// check for advertized namespaces in attributes
		if bytes.Equal(attr.Name.Prefix, bs("xmlns")) { // <- xmlns:prefix
			// this element introduces a new namespace that binds to a prefix
			// check if we already know this namespace by this or another prefix
			prefix := thiz.findPrefixForNamespace(attr.Value)
			if prefix != nil {
				if !bytes.Equal(prefix, attr.Name.Local) {
					// we don't know that particular prefix but we know that namespace
					// by another prefix, so establish a rewrite for the prefix
					thiz.addPrefixRewrite(attr.Name.Local, prefix)
				}
				// we don't need the attribute anymore because we already had a prefix
				continue
			}
			if !thiz.PreserveOriginalPrefixes {
				// wo don't know the prefix, but we want to rewrite it
				nextPrefixAlias := len(thiz.prefixAliases) / 2
				c := namespaceAliases[nextPrefixAlias : nextPrefixAlias+1]
				bsc := bs(c)
				thiz.addPrefixRewrite(attr.Name.Local, bsc)
				thiz.addNamespaceBinding(bsc, attr.Value)
				attr.Name.Local = bsc
			} else {
				thiz.addNamespaceBinding(attr.Name.Local, attr.Value)
			}
		} else if attr.Name.Prefix == nil && bytes.Equal(attr.Name.Local, bs("xmlns")) {
			// check if the element is already in that namespace, in which case
			// we can simply omit the namespace.
			currentNamespace := thiz.findNamespaceForPrefix(nil)
			if currentNamespace != nil {
				continue
			}
			// check if we already know a prefix for that namespace so that we
			// can use the prefix instead and drop the namespace declaration
			prefix := thiz.findPrefixForNamespace(attr.Value)
			if prefix != nil {
				// add prefix rewrite for "" -> prefix in order to remember
				// that any elements without a prefix get the new prefix now
				thiz.addPrefixRewrite(nil, prefix)
				t.Name.Prefix = prefix
				continue
			}
			// this element uses a new namespace in which all
			// unprefixed child elements will reside
			thiz.addNamespaceBinding(nil, attr.Value)
		}
		if i > j {
			t.Attr[j] = *attr
		}
		j++
	}
	t.Attr = t.Attr[:j]
}

func (thiz *NamespaceModifier) addNamespaceBinding(prefix, namespace []byte) {
	thiz.namespaces = append(thiz.namespaces, prefix, namespace)
	thiz.nsOffs[thiz.top]++
}

func (thiz *NamespaceModifier) addPrefixRewrite(original, prefix []byte) {
	thiz.prefixAliases = append(thiz.prefixAliases, original, prefix)
	thiz.prefixAliasesOffs[thiz.top]++
}

// NamespaceOfToken returns the effective namespace (as byte slice)
// of the pointed-to Token. The caller must make sure that the Token's fields/values
// will remain unmodified for the lexical scope of the XML element represented
// by that token, as per the documentation of EncoderMiddleware.EncodeToken.
func (thiz NamespaceModifier) NamespaceOfToken(t *Token) []byte {
	if t.Kind == TokenTypeInvalid {
		return nil
	}
	prefix := t.Name.Prefix
	if len(prefix) > 0 {
		alias := thiz.findPrefixAlias(t.Name.Prefix)
		if len(alias) > 0 {
			prefix = alias
		}
	}
	return thiz.findNamespaceForPrefix(prefix)
}
