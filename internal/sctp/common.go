package sctp

func getPadding(len int, multiple int) int {
	return (multiple - (len % multiple)) % multiple
}
