package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os/exec"
	"strings"
)

type Config struct {
	Camera Camera
	Server Server
}

func generateConfig() error {
	supportedDevices, err := checkDevices()
	if err != nil {
		return err
	}

	if len(supportedDevices) < 1 {
		return errors.New("no supported devices")
	}

	var camera Camera

	for _, device := range supportedDevices {
		out, err := exec.Command("v4l2-ctl", "--device="+device, "-D").Output()
		if err != nil {
			return err
		}

		data := processV4l2(string(out))
		rawName := strings.Split(data, "\n")[2]
		name := strings.Split(rawName, ":")[1]

		camera.Name = name
		camera.Width = 640
		camera.Height = 480
		camera.DevicePath = device
		//this example supports a stream from a single device
		//exit loop on finding one supported device
		break
	}

	server := Server{
		Port: ":5000",
	}

	config := Config{
		Camera: camera,
		Server: server,
	}

	j, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile("config.json", j, 0755); err != nil {
		return err
	}

	return nil
}
