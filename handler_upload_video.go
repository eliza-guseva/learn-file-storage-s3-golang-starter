package main

import (
	"context"
	"encoding/json"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"bytes"

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

	osFile, _ := os.CreateTemp("/tmp", videoID.String())
	io.Copy(osFile, rFile)
	osFile.Seek(0, 0)
	randomBytes := make([]byte, 32)
	_, _ = rand.Read(randomBytes)

	processedFilepath, err := processVideoForFastStart(osFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video", err)
		return
	}
	os.Remove(osFile.Name())
	osFile.Close()
	osFile, _ = os.Open(processedFilepath)
	
	defer os.Remove(osFile.Name())
	defer osFile.Close()

	// determine aspectRatio
	aspectRatio, err := getVideoAspectRatio(osFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}
	var prefix string
	switch aspectRatio {
		case "16:9":
			prefix = "landscape/"
		case "9:16":
			prefix = "portrait/"
		default:
			prefix = "other/"
	}

	output, err := cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key:    aws.String(prefix + hex.EncodeToString(randomBytes) + ".mp4"),
		Body:   osFile,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		slog.Error("Couldn't upload video", "error", err)
		return
	}
	slog.Info("uploaded video", "output", output)

	newURL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + prefix + hex.EncodeToString(randomBytes) + ".mp4"
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

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	slog.Info("ffprobe", "filepath", filepath)
	slog.Info("ffprobe", "cmd", cmd)
	cmd.Stdout = &bytes.Buffer{} 
	err := cmd.Run()
	if err != nil {
		slog.Error("ffprobe error", "error", err)
		return "", err
	}
	var jsonData map[string]interface{}
	json.Unmarshal(cmd.Stdout.(*bytes.Buffer).Bytes(), &jsonData)
	slog.Info("jsonData", "jsonData", jsonData)
	aspectRatio := jsonData["streams"].([]interface{})[0].(map[string]interface{})["display_aspect_ratio"].(string)
	if aspectRatio != "16:9" && aspectRatio != "9:16" {
		aspectRatio = "other"
	}
	return aspectRatio, nil
	
}

func processVideoForFastStart(filepath string) (string, error) {
	newFilepath := filepath + ".processed"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newFilepath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return newFilepath, nil
}

