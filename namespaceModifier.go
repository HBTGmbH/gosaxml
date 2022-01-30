package gosaxml

import "bytes"

// NamespaceModifier can be used to obtain information about the
// effective namespace of a decoded Token via NamespaceOfToken
// and to canonicalize/minify namespace declarations.
type NamespaceModifier struct {
	nsKeys [][]byte
	nsVals [][]byte
	nsOffs []int

	prefixAliasesKeys [][]byte
	prefixAliasesVals [][]byte
	prefixAliasesOffs []int

	openNames []Name

	top int
}

// NewNamespaceModifier creates a new NamespaceModifier and returns a pointer to it.
func NewNamespaceModifier() *NamespaceModifier {
	return &NamespaceModifier{
		nsKeys: make([][]byte, 0, 32),
		nsVals: make([][]byte, 0, 32),
		nsOffs: make([]int, 32),

		prefixAliasesKeys: make([][]byte, 0, 32),
		prefixAliasesVals: make([][]byte, 0, 32),
		prefixAliasesOffs: make([]int, 32),

		openNames: make([]Name, 32),
	}
}

// Reset resets this NamespaceModifier.
func (thiz *NamespaceModifier) Reset() {
	thiz.top = 0
}

func (thiz *NamespaceModifier) EncodeToken(t *Token) error {
	if t.Kind == TokenTypeStartElement {
		thiz.pushFrame()
		thiz.processNamespaces(t)
		thiz.processElementName(t)
		thiz.openNames[thiz.top] = t.Name
	} else if t.Kind == TokenTypeEndElement {
		thiz.processElementName(t)
		thiz.popFrame()
	}
	return nil
}

func (thiz *NamespaceModifier) processElementName(t *Token) {
	if t.Kind == TokenTypeStartElement {
		// Did we rewrite the prefix?
		prefix := thiz.findPrefixAlias(t.Name.Prefix)
		if prefix != nil {
			t.Name.Prefix = prefix
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
	for i := len(thiz.nsKeys) - 1; i >= 0; i-- {
		if bytes.Equal(thiz.nsKeys[i], prefix) {
			return thiz.nsVals[i]
		}
	}
	return nil
}

// findPrefixForNamespace finds the prefix which binds the given namespace (if any)
// This is the reverse operation of findNamespaceForPrefix.
func (thiz *NamespaceModifier) findPrefixForNamespace(namespace []byte) []byte {
	// scan all frames up to the top
	for i := len(thiz.nsKeys) - 1; i >= 0; i-- {
		if bytes.Equal(thiz.nsVals[i], namespace) {
			return thiz.nsKeys[i]
		}
	}
	return nil
}

// findPrefixAlias finds the alias for the given prefix.
// There is an alias for a given prefix it, during encoding, the prefix
// has been replaced with a (possibly) shorter alternative.
func (thiz *NamespaceModifier) findPrefixAlias(prefix []byte) []byte {
	// scan all frames up to the top
	for i := len(thiz.prefixAliasesKeys) - 1; i >= 0; i-- {
		if bytes.Equal(thiz.prefixAliasesKeys[i], prefix) {
			return thiz.prefixAliasesVals[i]
		}
	}
	return nil
}

func (thiz *NamespaceModifier) pushFrame() {
	thiz.top++
	thiz.nsOffs[thiz.top] = thiz.nsOffs[thiz.top-1]
	thiz.prefixAliasesOffs[thiz.top] = thiz.prefixAliasesOffs[thiz.top-1]
}

func (thiz *NamespaceModifier) popFrame() {
	thiz.top--
	off := thiz.nsOffs[thiz.top]
	thiz.nsKeys = thiz.nsKeys[:off]
	thiz.nsVals = thiz.nsVals[:off]
	off = thiz.prefixAliasesOffs[thiz.top]
	thiz.prefixAliasesKeys = thiz.prefixAliasesKeys[:off]
	thiz.prefixAliasesVals = thiz.prefixAliasesVals[:off]
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
			// wo don't know the prefix, but we want to rewrite it
			nextPrefixAlias := len(thiz.prefixAliasesKeys)
			c := namespaceAliases[nextPrefixAlias : nextPrefixAlias+1]
			bsc := bs(c)
			thiz.addPrefixRewrite(attr.Name.Local, bsc)
			thiz.addNamespaceBinding(bsc, attr.Value)
			attr.Name.Local = bsc
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
	thiz.nsKeys = append(thiz.nsKeys, prefix)
	thiz.nsVals = append(thiz.nsVals, namespace)
	thiz.nsOffs[thiz.top]++
}

func (thiz *NamespaceModifier) addPrefixRewrite(original, prefix []byte) {
	thiz.prefixAliasesKeys = append(thiz.prefixAliasesKeys, original)
	thiz.prefixAliasesVals = append(thiz.prefixAliasesVals, prefix)
	thiz.prefixAliasesOffs[thiz.top]++
}

// NamespaceOfToken returns the decoded effective namespace (as byte slice)
// of the provided Token. The byte slice will be from a pre-allocated pool
// in the Encoder and must not be accessed once the Token got out of scope in the Encoder.
func (thiz NamespaceModifier) NamespaceOfToken(t *Token) []byte {
	prefix := t.Name.Prefix
	if len(prefix) > 0 {
		alias := thiz.findPrefixAlias(t.Name.Prefix)
		if len(alias) > 0 {
			prefix = alias
		}
	}
	return thiz.findNamespaceForPrefix(prefix)
}
