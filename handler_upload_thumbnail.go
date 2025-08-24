package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

const MAXMEM = 10 << 20

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	slog.Info("handlerUploadThumbnail")
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

	file, mediaType, err := cfg.getFile(r, w)
	if err != nil {return}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't read thumbnail", err)
		return
	}
	videoDB := cfg.getVideoDB(videoID, userID, w)
	if videoDB.ID == uuid.Nil {return}

	imageString := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:" + mediaType + ";base64," + imageString
	// add the thumbnail to the videoDB)
	videoDB.ThumbnailURL = &dataURL
	fmt.Println("videoDB", videoDB)

	err = cfg.db.UpdateVideo(
		videoDB,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoDB)

}

func (cfg *apiConfig) getVideoDB(videoID uuid.UUID, userID uuid.UUID, w http.ResponseWriter) database.Video {
	videoDB, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return database.Video{}
	}
	if videoDB.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to do this", nil)
		return database.Video{}
	}
	return videoDB
}

func (cfg *apiConfig) getFile(r *http.Request, w http.ResponseWriter) (io.ReadCloser, string, error) {
	err := r.ParseMultipartForm(MAXMEM)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return nil, "", err
	}
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find thumbnail", err)
		return nil, "", err
	}
	return file, header.Header.Get("Content-Type"), nil
}
