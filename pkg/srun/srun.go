package srun

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"math"
)

const customAlpha = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"

var customBase64 = base64.NewEncoding(customAlpha).WithPadding('=')

func GetMD5(password, token string) string {
	h := hmac.New(md5.New, []byte(token))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

func GetSHA1(value string) string {
	h := sha1.New()
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

func GetBase64(s string) string {
	return customBase64.EncodeToString([]byte(s))
}

func GetXEncode(msg, key string) string {
	if msg == "" {
		return ""
	}
	pwd := sencode(msg, true)
	pwdk := sencode(key, false)

	if len(pwdk) < 4 {
		padding := make([]uint32, 4-len(pwdk))
		pwdk = append(pwdk, padding...)
	}

	n := len(pwd) - 1
	z := pwd[n]
	y := uint32(0)
	c := uint32(0x9E3779B9)
	q := int(math.Floor(6 + 52/float64(n+1)))
	d := uint32(0)
	e := uint32(0)

	for q > 0 {
		d += c
		e = (d >> 2) & 3
		p := 0
		for p < n {
			y = pwd[p+1]
			m := (z>>5 ^ y<<2) + ((y>>3 ^ z<<4) ^ (d ^ y))
			m += pwdk[(uint32(p)&3)^e] ^ z
			pwd[p] += m
			z = pwd[p]
			p++
		}
		y = pwd[0]
		m := (z>>5 ^ y<<2) + ((y>>3 ^ z<<4) ^ (d ^ y))
		m += pwdk[(uint32(p)&3)^e] ^ z
		pwd[n] += m
		z = pwd[n]
		q--
	}

	return lencode(pwd, false)
}

func sencode(msg string, key bool) []uint32 {
	l := len(msg)
	pwd := make([]uint32, 0, l/4+1)
	for i := 0; i < l; i += 4 {
		var val uint32
		val |= ordat(msg, i)
		val |= ordat(msg, i+1) << 8
		val |= ordat(msg, i+2) << 16
		val |= ordat(msg, i+3) << 24
		pwd = append(pwd, val)
	}
	if key {
		pwd = append(pwd, uint32(l))
	}
	return pwd
}

func lencode(msg []uint32, key bool) string {
	l := len(msg)
	ll := (l - 1) << 2
	if key {
		m := msg[l-1]
		if m < uint32(ll-3) || m > uint32(ll) {
			return ""
		}
		ll = int(m)
	}
	res := make([]byte, l*4)
	for i := range l {
		res[i*4] = byte(msg[i] & 0xff)
		res[i*4+1] = byte(msg[i] >> 8 & 0xff)
		res[i*4+2] = byte(msg[i] >> 16 & 0xff)
		res[i*4+3] = byte(msg[i] >> 24 & 0xff)
	}
	if key {
		return string(res[:ll])
	}
	return string(res)
}

func ordat(msg string, idx int) uint32 {
	if len(msg) > idx {
		return uint32(msg[idx])
	}
	return 0
}