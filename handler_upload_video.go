package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1 << 30)
	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	userID := cfg.AuthUser(w, r)
	if userID == uuid.Nil { return }

	videoDB, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if videoDB.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to do this", nil)
		return
	}
	rFile, rHeader, err := r.FormFile("video")
	defer rFile.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video", err)
		return
	}
	if rHeader.Header.Get("Content-Type") != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Wrong Video Type, use mp4", err)
		return
	}

	osFile, err := os.CreateTemp("/tmp", videoID.String())
	defer os.Remove(osFile.Name())
	defer osFile.Close()
	io.Copy(osFile, rFile)
	osFile.Seek(0, 0)
	randomBytes := make([]byte, 32)
	_, _ = rand.Read(randomBytes)
	output, err := cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key:    aws.String(hex.EncodeToString(randomBytes) + ".mp4"),
		Body:   osFile,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		slog.Error("Couldn't upload video", "error", err)
		return
	}
	slog.Info("uploaded video", "output", output)

	newURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + hex.EncodeToString(randomBytes) + ".mp4"
	videoDB.VideoURL = &newURL
	err = cfg.db.UpdateVideo(
		videoDB,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoDB)


}
