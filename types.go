package gosaxml

// Name is a name with a possible prefix like "xmlns:blubb"
// or simply without prefix like "a"
type Name struct {
	Local  []byte
	Prefix []byte
}

// Attr is an attribute of an element.
// Only tokens of type TokenTypeStartElement can have attributes.
type Attr struct {
	Name        Name
	Value       []byte
	SingleQuote bool
}

// constants for Token.Kind
const (
	TokenTypeInvalid = iota
	TokenTypeStartElement
	TokenTypeEndElement
	TokenTypeProcInst
	TokenTypeDirective
	TokenTypeTextElement
	TokenTypeCharData
)

// Token represents the union of all possible token types
// with their respective information.
type Token struct {
	// only for TokenTypeStartElement, TokenTypeEndElement and TokenTypeProcInst
	Name Name

	// only for TokenTypeStartElement
	Attr []Attr

	// only for TokenTypeDirective, TokenTypeTextElement, TokenTypeCharData and TokenTypeProcInst
	ByteData []byte

	Kind byte
}
