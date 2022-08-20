package gosaxml

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOpenAngleBracket16(t *testing.T) {
	assert.Equal(t, byte(3), openAngleBracket16(at(slice(16, ' '), 3, '<')))
	assert.Equal(t, byte(15), openAngleBracket16(at(slice(16, ' '), 15, '<')))
	assert.Equal(t, byte(16), openAngleBracket16(slice(16, ' ')))
}

func TestOnlySpaces16(t *testing.T) {
	assert.Equal(t, byte(16), onlySpaces16(slice(16, ' ')))
	assert.Equal(t, byte(16), onlySpaces16(slice(16, '\t')))
	assert.Equal(t, byte(16), onlySpaces16(slice(16, '\n')))
	assert.Equal(t, byte(16), onlySpaces16(slice(16, '\t')))
	assert.Equal(t, byte(3), onlySpaces16(at(slice(16, '\n'), 3, ':')))
	assert.Equal(t, byte(15), onlySpaces16(at(slice(16, ' '), 15, ':')))
	assert.Equal(t, byte(3), onlySpaces16(at(at(slice(16, ' '), 3, 0xC2), 4, 0xA7)))
}

func TestOnlySpaces32(t *testing.T) {
	assert.Equal(t, byte(32), onlySpaces32(slice(32, ' ')))
	assert.Equal(t, byte(32), onlySpaces32(slice(32, '\t')))
	assert.Equal(t, byte(32), onlySpaces32(slice(32, '\n')))
	assert.Equal(t, byte(32), onlySpaces32(slice(32, '\t')))
	assert.Equal(t, byte(3), onlySpaces32(at(slice(32, '\n'), 3, ':')))
	assert.Equal(t, byte(15), onlySpaces32(at(slice(32, ' '), 15, ':')))
	assert.Equal(t, byte(31), onlySpaces32(at(slice(32, ' '), 31, ':')))
	assert.Equal(t, byte(3), onlySpaces32(at(at(slice(32, ' '), 3, 0xC2), 4, 0xA7)))
}

func TestSeparator32(t *testing.T) {
	assert.Equal(t, byte(32), seperator32(slice(32, 'a')))
	assert.Equal(t, byte(15), seperator32(at(slice(32, 'a'), 15, ':')))
	assert.Equal(t, byte(31), seperator32(at(slice(32, 'a'), 31, ':')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '/')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '>')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '=')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, ' ')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '\t')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '\n')))
	assert.Equal(t, byte(5), seperator32(at(slice(32, 'a'), 5, '\r')))
}

func slice(len int, v uint8) []uint8 {
	s := make([]uint8, len)
	for i := range s {
		s[i] = v
	}
	return s
}

func at(s []uint8, i int, v uint8) []uint8 {
	s[i] = v
	return s
}
