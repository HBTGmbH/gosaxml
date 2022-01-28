package gosaxml

import "bytes"

// frame is created for each start XML element and contains
// the potential namespaces and prefixes created for that element
type frame struct {
	ns            map[string][]byte
	prefixAliases map[string][]byte
	newPrefixes   int
	openName      Name
}

type NamespaceModifier struct {
	// the frames (defining namespaces and prefix rewrites)
	frames []frame

	// pointer to the current active frame
	f *frame

	// the next free/available prefix alias
	nextPrefix int

	// the current "cursor" into the frames slice
	top int
}

// Reset resets a Frame
func (thiz *frame) Reset() {
	thiz.newPrefixes = 0
	thiz.ns = nil
	thiz.prefixAliases = nil
}

func NewNamespaceModifier() *NamespaceModifier {
	return &NamespaceModifier{
		frames: make([]frame, 32),
	}
}

func (thiz *NamespaceModifier) Reset() {
	thiz.top = 0
}

func (thiz *NamespaceModifier) EncodeToken(t *Token) error {
	if t.Kind == TokenTypeStartElement {
		thiz.pushFrame()
		thiz.processNamespaces(t)
		thiz.processElementName(t)
		thiz.f.openName = t.Name
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
		t.Name = thiz.f.openName
	}
}

// findNamespaceForPrefix finds the namespace bound to the given prefix (if any)
// within the stack frame of the current element
// This is the reverse operation of findPrefixForNamespace.
func (thiz *NamespaceModifier) findNamespaceForPrefix(prefix []byte) []byte {
	// scan all frames up to the top
	for t := thiz.top - 1; t >= 0; t-- {
		f := thiz.frames[t]
		if f.ns == nil {
			continue
		}
		ns := f.ns[string(prefix)]
		if ns != nil {
			return ns
		}
	}
	return nil
}

// findPrefixForNamespace finds the prefix which binds the given namespace (if any)
// This is the reverse operation of findNamespaceForPrefix.
func (thiz *NamespaceModifier) findPrefixForNamespace(namespace []byte) []byte {
	if thiz.top <= 0 {
		return nil
	}
	for t := thiz.top - 1; t >= 0; t-- {
		f := thiz.frames[t]
		if f.ns == nil {
			continue
		}
		for k, v := range f.ns {
			if bytes.Equal(v, namespace) {
				return bs(k)
			}
		}
	}
	return nil
}

// findPrefixAlias finds the alias for the given prefix.
// There is an alias for a given prefix it, during encoding, the prefix
// has been replaced with a (possibly) shorter alternative.
func (thiz *NamespaceModifier) findPrefixAlias(prefix []byte) []byte {
	if thiz.top <= 0 {
		return nil
	}
	for t := thiz.top - 1; t >= 0; t-- {
		f := thiz.frames[t]
		if f.prefixAliases == nil {
			continue
		}
		v := f.prefixAliases[string(prefix)]
		if v != nil {
			return v
		}
	}
	return nil
}

func (thiz *NamespaceModifier) pushFrame() {
	thiz.frames[thiz.top].Reset()
	thiz.f = &thiz.frames[thiz.top]
	thiz.top++
}

func (thiz *NamespaceModifier) popFrame() {
	thiz.top--
	thiz.nextPrefix -= thiz.frames[thiz.top].newPrefixes
	if thiz.top > 0 {
		thiz.f = &thiz.frames[thiz.top-1]
	} else {
		thiz.f = nil
	}
}

// processNamespaces scans the attributes of the given token for namespace declarations,
// either with or without a binding prefix and possibly re-assigns prefixes to other existing
// or new aliases and drops redundant namespace declarations.
func (thiz *NamespaceModifier) processNamespaces(t *Token) {
	var ns map[string][]byte
	var prefixRewrites map[string][]byte
	var newAttributes []Attr
	for _, attr := range t.Attr {
		// check for advertized namespaces in attributes
		if bytes.Equal(attr.Name.Prefix, bs("xmlns")) { // <- xmlns:prefix
			// this element introduces a new namespace that binds to a prefix
			// check if we already know this namespace by this or another prefix
			prefix := thiz.findPrefixForNamespace(attr.Value)
			if prefix != nil {
				if !bytes.Equal(prefix, attr.Name.Local) {
					// we don't know that particular prefix but we know that namespace
					// by another prefix, so establish a rewrite for the prefix
					if prefixRewrites == nil {
						prefixRewrites = make(map[string][]byte)
					}
					prefixRewrites[string(attr.Name.Local)] = prefix
					// check if that element itself belongs to that prefix
					if bytes.Equal(t.Name.Prefix, attr.Name.Local) {
						// rewrite element prefix
						t.Name.Prefix = prefix
					}
				}
				// we don't need the attribute anymore because we already had a prefix
				continue
			}
			// wo don't know the prefix, but we want to rewrite it
			c := namespaceAliases[thiz.nextPrefix : thiz.nextPrefix+1]
			thiz.f.newPrefixes++
			thiz.nextPrefix++
			if prefixRewrites == nil {
				prefixRewrites = make(map[string][]byte)
			}
			prefixRewrites[string(attr.Name.Local)] = bs(c)
			if ns == nil {
				ns = make(map[string][]byte)
			}
			attr.Name.Local = bs(c)
			ns[c] = append([]byte(nil), attr.Value...)
		} else if attr.Name.Prefix == nil && bytes.Equal(attr.Name.Local, bs("xmlns")) {
			// check if the element is already in that namespace, in which case
			// we can simply omit the namespace.
			currentNamespace := thiz.findNamespaceForPrefix([]byte{})
			if currentNamespace != nil {
				continue
			}
			// check if we already know a prefix for that namespace so that we
			// can use the prefix instead and drop the namespace declaration
			prefix := thiz.findPrefixForNamespace(attr.Value)
			if prefix != nil {
				t.Name.Prefix = prefix
				continue
			}
			if ns == nil {
				ns = make(map[string][]byte)
			}
			// this element uses a new namespace in which all
			// unprefixed child elements will reside
			ns[""] = append([]byte(nil), attr.Value...)
		}
		newAttributes = append(newAttributes, attr)
	}
	t.Attr = newAttributes

	// store established new namespaces and prefix rewrites
	thiz.f.ns = ns
	thiz.f.prefixAliases = prefixRewrites
}

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
