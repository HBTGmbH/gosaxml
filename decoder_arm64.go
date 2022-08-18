package gosaxml

func (thiz *decoder) skipWhitespaces(b byte) (byte, error) {
	return thiz.skipWhitespacesGeneric(b)
}

func (thiz *decoder) decodeText(t *Token) (bool, error) {
	return thiz.decodeTextGeneric(t)
}
