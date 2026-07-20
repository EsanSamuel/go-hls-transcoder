package models

import (
	"io"
	"os"

	"github.com/EsanSamuel/go-hls-transcoder/internal/entity"
)

type VideoQuality struct {
	Name    string
	Width   int
	Height  int
	Bitrate string
	Maxrate string
	Bufsize string
}

type FFmpegService struct {
	videoQualities []VideoQuality
	cpuCoreRequest float32
	cpuCoreLimit   float32
}

type Stream struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CodecType string `json:"codec_type"`
}

type VideoData struct {
	Streams []Stream `json:"streams"`
}

type VideoStorage interface {
	Save(reader io.Reader, path ...string) (entity.Path, error) // Save video data to storage
	Open(path ...string) (io.ReadCloser, error)                 // Open video file for reading
	GetPath(path ...string) (entity.Path, error)                // Get absolute path of stored video file
}

type FFmpeg interface {
	Transcode(input entity.Path, isPortrait bool) error          // Transcode video data
	GetVideoDetails(path entity.Path) (*VideoData, error)        // Get video details
	GetSnapshot(id string, input entity.Path) (*os.File, error)  // Extract snapshot from video
	extractAudio(input entity.Path, id string) (*os.File, error) // Extract audio from video
}

type VideoUseCase struct {
	Storage VideoStorage // Interface for saving and retrieving video files
	FFmpeg  FFmpeg       // Interface for video processing (transcoding, probing)
}

type VideoController struct {
	videoUseCase *VideoUseCase // Use case for handling video-related operations
}

type FileSystemStorage struct {
	baseDir entity.Path
}
