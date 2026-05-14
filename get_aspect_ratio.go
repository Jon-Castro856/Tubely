package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	type videoRatio struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	command := exec.Command("ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filepath)

	var b bytes.Buffer
	command.Stdout = &b

	err := command.Run()
	if err != nil {
		return "", err
	}

	var vidData videoRatio
	err = json.Unmarshal(b.Bytes(), &vidData)
	if err != nil {
		return "", err
	}

	ratio := float32(vidData.Width) / float32(vidData.Height)

	if ratio > 1 {
		return "16:9", nil
	}

	return "9:16", nil
}
