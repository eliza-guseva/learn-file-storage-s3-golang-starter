package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerThumbnailGet(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	videoDB, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if videoDB.ThumbnailURL == nil {
		respondWithError(w, http.StatusNotFound, "Thumbnail not found", nil)
		return
	}

	thumbnailURL := *videoDB.ThumbnailURL
	mediaType := strings.Split(thumbnailURL, ".")[1]
	fmt.Println("in handlerGetThumbnail, thumbnailURL", thumbnailURL)
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(*videoDB.ThumbnailURL)))
	// read the image from thumbnailURL
	imageDataReader := bytes.NewReader([]byte(thumbnailURL))
	_, err = io.Copy(w, imageDataReader)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing response", err)
		return
	}
}
