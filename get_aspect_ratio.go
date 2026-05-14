package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	type videoRatio struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
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
		fmt.Println("error executing command")
		return "", err
	}

	var vidData videoRatio
	err = json.Unmarshal(b.Bytes(), &vidData)
	if err != nil {
		return "", err
	}
	width := vidData.Streams[0].Width
	height := vidData.Streams[0].Height

	divisor := GCD(width, height)
	fmt.Println(divisor)

	w := width / divisor
	h := height / divisor

	if w > h {
		return "landscape", nil
	}

	return "portrait", nil
}

func processVideoForFastStart(filepath string) (string, error) {
	newPath := filepath + ".processing"
	command := exec.Command("ffmpeg",
		"-i",
		filepath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		newPath)

	var b bytes.Buffer
	command.Stdout = &b

	err := command.Run()
	if err != nil {
		fmt.Println("error executing command")
		return "", err
	}

	return newPath, nil
}

func GCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
