package srtp

import "bytes"

// Check if buffers match, if not allocate a new buffer and return it
func allocateIfMismatch(dst, src []byte) []byte {
	if dst == nil {
		dst = make([]byte, len(src))
		copy(dst, src)
	} else if !bytes.Equal(dst, src) { // bytes.Equal returns on ref equality, no optimization needed
		extraNeeded := len(src) - len(dst)
		if extraNeeded > 0 {
			dst = append(dst, make([]byte, extraNeeded)...)
		} else if extraNeeded < 0 {
			dst = dst[:len(dst)+extraNeeded]
		}

		copy(dst, src)
	}

	return dst
}
