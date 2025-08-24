package main

import (
	"fmt"
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

	mediaType := strings.Split(*videoDB.ThumbnailURL, ";")[0]
	data := strings.Split(*videoDB.ThumbnailURL, ",")[1]

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	_, err = w.Write([]byte(data))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing response", err)
		return
	}
}
