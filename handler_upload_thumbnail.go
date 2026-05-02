package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to Parse Multipart Form", err)
		return
	}

	fileData, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No data found in header", err)
	}

	contentType := fileHeader.Header.Get("Content-Type")

	imageData, err := io.ReadAll(fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read data", err)
		return
	}

	metaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to get metadata", err)
		return
	}
	fmt.Println(metaData)
	fmt.Println(userID)

	if metaData.CreateVideoParams.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized user", nil)
		return
	}

	tn := thumbnail{
		data:      imageData,
		mediaType: contentType,
	}

	videoThumbnails[metaData.ID] = tn
	newURL := "http://localhost:8091/api/thumbnails/" + metaData.ID.String()
	metaData.ThumbnailURL = &newURL

	err = cfg.db.UpdateVideo(metaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, metaData)
}
