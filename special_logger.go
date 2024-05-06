package webrtc

import "fmt"

const specialLogEnabled = false

func specialLog(toLog ...interface{}) {
	if specialLogEnabled {
		fmt.Println(toLog...)
	}
}
