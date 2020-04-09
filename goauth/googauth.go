package goauth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
)

func ComputeCode(secret string, value int64) int {

	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return -1
	}

	hash := hmac.New(sha1.New, key)
	err = binary.Write(hash, binary.BigEndian, value)
	if err != nil {
		return -1
	}
	h := hash.Sum(nil)

	offset := h[19] & 0x0f

	code := binary.BigEndian.Uint32(h[offset : offset+4])

	return int(code & 0xffffff)
}

type OTPConfig struct {
	Secret        string
}


func (c *OTPConfig) ShowCode(t0 int64) int {
	return ComputeCode(c.Secret, t0);
}


