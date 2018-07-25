package sctp

const (
	paddingMultiple = 4
)

func getPadding(len int) int {
	return (paddingMultiple - (len % paddingMultiple)) % paddingMultiple
}
