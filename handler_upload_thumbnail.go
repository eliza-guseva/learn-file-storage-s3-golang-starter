package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

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

	videoDB := cfg.getVideoDB(videoID, userID, w)
	if videoDB.ID == uuid.Nil {return}

	fname := writeThumbnailToAssets(videoID, mediaType, w, file)
	if fname == "" {return}
	// add the thumbnail to the videoDB)
	thumbnailURL := "http://localhost:8091/" + fname
	videoDB.ThumbnailURL = &thumbnailURL
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
	mediaType := header.Header.Get("Content-Type")
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", nil)
		return nil, "", fmt.Errorf("invalid media type: %s", mediaType)
	}
	return file, mediaType, nil
}

func writeThumbnailToAssets(videoID uuid.UUID, mediaType string, w http.ResponseWriter, file io.ReadCloser) string {
	fname := "assets/" + videoID.String() + "." + strings.Split(mediaType, "/")[1]
	osFile, err := os.Create(fname)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return ""
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy file", err)
		return ""
	}
	return fname
}
