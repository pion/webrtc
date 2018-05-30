package main

import (
	"encoding/base64"
	"fmt"
)

func main() {
	a := generateVP8OnlyAnswer().Marshal()
	fmt.Println(a)
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(a)))
}
