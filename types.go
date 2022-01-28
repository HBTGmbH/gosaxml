package gosaxml

// Name is a name with a possible prefix like "xmlns:blubb"
// or simply without prefix like "a"
type Name struct {
	Local  []byte
	Prefix []byte
}

// Attr is an attribute of an element.
// Only elements of type TokenTypeStartElement can have attributes.
type Attr struct {
	Name        Name
	SingleQuote bool
	Value       []byte
}

// constants for Token.Kind
const (
	TokenTypeStartElement = iota
	TokenTypeEndElement
	TokenTypeProcInst
	TokenTypeDirective
	TokenTypeTextElement
	TokenTypeCharData
)

// Token represents the union of all possible token types
// and their respective information.
type Token struct {
	Kind byte

	// only for TokenTypeStartElement and TokenTypeEndElement
	Name Name

	// only for TokenTypeStartElement
	Attr []Attr

	// only for TokenTypeDirective, TokenTypeTextElement, TokenTypeCharData and TokenTypeProcInst
	ByteData []byte
}
