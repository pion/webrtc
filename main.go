package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	fmt.Print("base64 encoded Session Description: ")
	text, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Println(text)

	a := generateVP8OnlyAnswer().Marshal()
	fmt.Println(a)
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(a)))
	select {}
}
