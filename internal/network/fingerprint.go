package network

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
)

// TODO: Move to DTLS?

type digestFunc func([]byte) []byte

var FingerprintAlgoritms = []string{"sha-256"}

var digesters = map[string]digestFunc{
	// "md2":     nil, // [RFC3279]
	// "md5":     nil, // [RFC3279]
	// "sha-1":   nil, // [RFC3279]
	// "sha-224": nil, // [RFC4055]
	"sha-256": sha256Digest, // [RFC4055]
	// "sha-384": nil, // [RFC4055]
	// "sha-512": nil, // [RFC4055]
}

func sha256Digest(b []byte) []byte {
	hash := sha256.Sum256(b)
	return hash[:]
}

func Fingerprint(cert *x509.Certificate, algo string) (string, error) {
	digester, ok := digesters[algo]
	if !ok {
		return "", fmt.Errorf("Unknown fingerprinting algorithm %s", algo)
	}

	digest := []byte(fmt.Sprintf("%x", digester(cert.Raw)))

	digestlen := len(digest)
	if digestlen == 0 {
		return "", nil
	}
	if digestlen%2 != 0 {
		return "", errors.New("invalid fingerprint length")
	}
	res := make([]byte, digestlen>>1+digestlen-1)

	pos := 0
	for i, c := range digest {
		res[pos] = c
		pos++
		if (i)%2 != 0 && i < digestlen-1 {
			res[pos] = byte(':')
			pos++
		}
	}

	return string(res), nil
}
