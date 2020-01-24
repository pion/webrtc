package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func checkDevices() ([]string, error) {
	devices, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, err
	}

	if len(devices) < 1 {
		return nil, errors.New("no devices found")
	}

	supportedDevices := []string{}

	for id, device := range devices {
		out, err := exec.Command("v4l2-ctl", "--device="+device, "--list-formats").Output()
		if err != nil {
			return nil, err
		}

		data := processV4l2(string(out))
		formats := strings.Split(data, "\n")

		for _, format := range formats {
			details := strings.Split(format, ":")

			for _, detail := range details {
				if detail == "H.264" {
					supportedDevices = append(supportedDevices, "/dev/video"+strconv.Itoa(id))
				}
			}
		}
	}

	return supportedDevices, nil
}

func processV4l2(input string) string {
	data := strings.Replace(input, "\n\n", "\n", -1)
	data = strings.Replace(data, "\n\t", "\n", -1)
	data = strings.Replace(data, "\n\n\t", "\n", -1)
	data = strings.Replace(data, " ", "", -1)

	return data
}
