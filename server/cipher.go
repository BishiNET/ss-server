package server

import (
	"crypto/md5"
	"errors"
	"net"
	"sort"
	"strings"
)

// ErrCipherNotSupported occurs when a cipher is not supported (likely because of security concerns).
var ErrCipherNotSupported = errors.New("cipher not supported")

const (
	aeadAes128Gcm        = "AEAD_AES_128_GCM"
	aeadAes256Gcm        = "AEAD_AES_256_GCM"
	aeadChacha20Poly1305 = "AEAD_CHACHA20_POLY1305"
)

// List of AEAD ciphers: key size in bytes and constructor
var aeadList = map[string]struct {
	KeySize int
	New     func([]byte) (Cipher, error)
}{
	aeadAes128Gcm:        {16, AESGCM},
	aeadAes256Gcm:        {32, AESGCM},
	aeadChacha20Poly1305: {32, Chacha20Poly1305},
}

// ListCipher returns a list of available cipher names sorted alphabetically.
func ListCipher() []string {
	var l []string
	for k := range aeadList {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

func kdf(password string, keyLen int) []byte {
	var b, prev []byte
	h := md5.New()
	for len(b) < keyLen {
		h.Write(prev)
		h.Write([]byte(password))
		b = h.Sum(b)
		prev = b[len(b)-h.Size():]
		h.Reset()
	}
	return b[:keyLen]
}

// PickCipher returns a Cipher of the given name. Derive key from password if given key is empty.
func PickCipher(name string, key []byte, password string, u *User) (*aeadCipher, error) {
	name = strings.ToUpper(name)

	switch name {
	case "CHACHA20-IETF-POLY1305":
		name = aeadChacha20Poly1305
	case "AES-128-GCM":
		name = aeadAes128Gcm
	case "AES-256-GCM":
		name = aeadAes256Gcm
	}

	if choice, ok := aeadList[name]; ok {
		if len(key) == 0 {
			key = kdf(password, choice.KeySize)
		}
		if len(key) != choice.KeySize {
			return nil, KeySizeError(choice.KeySize)
		}
		aead, err := choice.New(key)
		return &aeadCipher{aead, u}, err
	}

	return nil, ErrCipherNotSupported
}

type aeadCipher struct {
	Cipher
	*User
}

func (aead *aeadCipher) StreamConn(c net.Conn) net.Conn { return NewConn(c, aead.Cipher, aead.User) }
func (aead *aeadCipher) PacketConn(c net.PacketConn) net.PacketConn {
	return NewPacketConn(c, aead.Cipher, aead.User)
}
