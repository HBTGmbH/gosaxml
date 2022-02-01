package gosaxml

import (
	"errors"
	"io"
)

type bufreader struct {
	buf [4096]byte
	rd  io.Reader
	r   int
	w   int
}

func newBufReader(r io.Reader) *bufreader {
	return &bufreader{
		rd: r,
	}
}

func (b *bufreader) read0() error {
	if b.r > 0 {
		copy(b.buf[:], b.buf[b.r:b.w])
		b.r = 0
		b.w -= b.r
	}
	n, err := b.rd.Read(b.buf[b.w:])
	b.w += n
	if err != nil {
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

func (b *bufreader) unreadByte() error {
	if b.r == 0 {
		return errors.New("cannot perform unread")
	}
	b.r--
	return nil
}

func (b *bufreader) reset(r io.Reader) {
	b.rd = r
	b.r = 0
	b.w = 0
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
