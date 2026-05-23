package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to get metadata", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized user", nil)
		return
	}

	fileData, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No data found in header", err)
		return
	}
	defer fileData.Close()

	contentType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error acquiring content type", err)
		return
	}
	if contentType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Content type not valid", nil)
		return
	}

	rng := make([]byte, 32)
	_, err = rand.Read(rng)
	rngString := base64.RawURLEncoding.EncodeToString(rng)

	//initial temp file creation
	file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create tmp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer file.Close()

	_, err = io.Copy(file, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy tmp file", err)
		return
	}

	orientation, err := getVideoAspectRatio(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching video aspect ratio", err)
		return
	}

	newFilePath, err := processVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error Pre-processing video", err)
		return
	}

	//new temp file for processed faststart video
	processedFile, err := os.Open(newFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening tmp file", err)
	}
	defer os.Remove(newFilePath)
	defer processedFile.Close()

	_, err = io.Copy(file, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy tmp file", err)
		return
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Internal error", err)
		return
	}
	key := orientation + "/" + rngString + ".mp4"
	bucketParams := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processedFile,
		ContentType: &contentType,
	}
	_, err = cfg.client.PutObject(r.Context(), &bucketParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload video to s3 Bucket", err)
		return
	}

	if video.VideoURL == nil {
		bucket := cfg.s3Bucket
		keyURL := bucket + "," + key
		video.VideoURL = &keyURL
		video, err = cfg.dbVideoToSignedVideo(video)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error creating Presigned URL", err)
			return
		}
	}
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	keyString := strings.Split(*video.VideoURL, ",")
	if len(keyString) < 2 {
		return video, fmt.Errorf("Malformed video url")
	}
	bucket := keyString[0]
	key := keyString[1]

	presignedURL, err := generatePresignedURL(cfg.client, bucket, key, time.Minute*15)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL
	return video, nil
}

func generatePresignedURL(s3client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3client)

	params := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	request, err := presignClient.PresignGetObject(context.Background(), params, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return request.URL, nil
}
