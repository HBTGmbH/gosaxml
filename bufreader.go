package gosaxml

import (
	"encoding/binary"
	"io"
)

type bufreader struct {
	buf [4096]byte
	rd  io.Reader
	r   int
	w   int
}

func (b *bufreader) read0() error {
	if b.r > 0 {
		copy(b.buf[:], b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}
	n, err := b.rd.Read(b.buf[b.w:])
	b.w += n
	if n <= 0 && err != nil {
		return err
	}
	return nil
}

func (b *bufreader) readByte() (byte, error) {
	for b.r == b.w {
		err := b.read0()
		if err != nil {
			return 0, err
		}
	}
	c := b.buf[b.r]
	b.r++
	return c, nil
}

func (b *bufreader) unreadByte() {
	b.r--
}

func (b *bufreader) unreadBytes(n int) {
	b.r -= n
}

func (b *bufreader) readUint64() (uint64, int, error) {
	if b.r+8 > b.w {
		_ = b.read0()
	}
	n := b.w - b.r
	if n > 8 {
		n = 8
	}
	u := binary.LittleEndian.Uint64(b.buf[b.r : b.r+8])
	b.r += n
	return u, n, nil
}

func (b *bufreader) reset(r io.Reader) {
	b.rd = r
	b.r = 0
	b.w = 0
}

func (b *bufreader) discardBuffer() {
	b.r = b.w
}

func (b *bufreader) discard(n int) (int, error) {
	for b.r+n > b.w {
		err := b.read0()
		if err != nil {
			return 0, err
		}
	}
	b.r += n
	return n, nil
}
