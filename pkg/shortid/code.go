package shortid

import (
	"fmt"
	"io"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type CodeGenerator struct {
	random  io.Reader
	counter uint16
}

var _ Generator = (*CodeGenerator)(nil)

func NewCodeGenerator(random io.Reader) *CodeGenerator {
	return &CodeGenerator{random: random}
}

func (g *CodeGenerator) Next() (string, error) {
	var buf [3]byte

	if _, err := io.ReadFull(g.random, buf[:]); err != nil {
		return "", fmt.Errorf("shortid: read random bytes: %w", err)
	}

	r := uint64(buf[0])<<16 |
		uint64(buf[1])<<8 |
		uint64(buf[2])

	r &= (1 << 19) - 1

	value := r<<16 | uint64(g.counter)

	var code [6]byte
	for i := len(code) - 1; i >= 0; i-- {
		code[i] = alphabet[value%62]
		value /= 62
	}

	g.counter++
	return string(code[:]), nil
}
