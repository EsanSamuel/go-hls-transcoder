package video

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AssemblyAI/assemblyai-go-sdk"
	"github.com/EsanSamuel/go-hls-transcoder/internal/entity"

	"github.com/EsanSamuel/go-hls-transcoder/internal/jobs"
	"github.com/EsanSamuel/go-hls-transcoder/internal/storage"
	"github.com/google/uuid"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
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
	VideoQualities []VideoQuality
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

type FFmpeg interface {
	Transcode(input entity.Path, isPortrait bool) error          // Transcode video data
	GetVideoDetails(path entity.Path) (*VideoData, error)        // Get video details
	GetSnapshot(id string, input entity.Path) (*os.File, error)  // Extract snapshot from video
	extractAudio(input entity.Path, id string) (*os.File, error) // Extract audio from video
}

type VideoUseCase struct {
	Storage storage.VideoStorage // Interface for saving and retrieving video files
	FFmpeg  FFmpeg               // Interface for video processing (transcoding, probing)
}

type VideoController struct {
	videoUseCase *VideoUseCase // Use case for handling video-related operations
}

var VideoQualities = []VideoQuality{
	{"1080p", 1920, 1080, "4500k", "4700k", "6000k"},
	{"720p", 1280, 720, "2500k", "2675k", "3750k"},
	{"480p", 854, 480, "1000k", "1075k", "1500k"},
	{"360p", 640, 360, "600k", "650k", "900k"},
	{"240p", 426, 240, "400k", "450k", "600k"},
	{"144p", 256, 144, "250k", "275k", "400k"},
}

func (vq VideoQuality) ScaleHorizonatally() string {
	return fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", vq.Width, vq.Height)
}

func (vq VideoQuality) ScaleVertically() string {
	return fmt.Sprintf("scale='min(%d,iw*%d/ih)':-1", vq.Width, vq.Height)
}

func (vq VideoQuality) LandScape() string {
	return fmt.Sprintf("%dx%d", vq.Width, vq.Height)
}

func (vq VideoQuality) Portrait() string {
	return fmt.Sprintf("%dx%d", vq.Height, vq.Width)
}

func (v VideoData) IsPortrait() bool {
	for _, stream := range v.Streams {
		if stream.CodecType == "video" {
			return stream.Height > stream.Width
		}
	}
	return false
}

func (s *FFmpegService) GetVideoDetails(path entity.Path) (*VideoData, error) {
	data, err := ffmpeg_go.Probe(path.String())
	if err != nil {
		return nil, fmt.Errorf("failed to probe video: %w", err)
	}
	var result VideoData
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal video data: %w", err)
	}
	return &result, nil
}

func (s *FFmpegService) Transcode(input entity.Path, isPortrait bool) error {
	for _, q := range s.VideoQualities {
		outputPath := input.Parent().String()
		qualityDir := filepath.Join(outputPath, "normal_hls", q.Name)
		if err := os.MkdirAll(qualityDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output dir %s: %w", qualityDir, err)
		}

		inputPath := filepath.ToSlash(input.String())
		segmentDir := filepath.ToSlash(qualityDir)
		playlistPath := segmentDir + "/index.m3u8"
		segmentPath := segmentDir + "/%03d.ts"
		scalefilter := q.ScaleHorizonatally()
		if isPortrait {
			scalefilter = q.ScaleVertically()
		}

		cmd := ffmpeg_go.Input(inputPath).Output(playlistPath, s.getFFmepegArgs(q, segmentPath, []string{scalefilter, q.LandScape()}))

		err := cmd.OverWriteOutput().WithOutput(nil, os.Stdout).Run()
		if err != nil {
			return fmt.Errorf("ffmpeg failed for quality %s: %w", q.Name, err)
		}
	}
	if err := s.generateMasterPlaylist(input.Parent()); err != nil {
		return fmt.Errorf("failed to generate master playlist: %w", err)
	}
	return nil
}

func (s *FFmpegService) getFFmepegArgs(q VideoQuality, segmentPath string, filters []string) ffmpeg_go.KwArgs {
	return ffmpeg_go.KwArgs{
		"c:v":                  "h264",                          // Use H.264 video codec
		"profile:v":            "main",                          // Set video encoding profile to "main" for broad compatibility
		"crf":                  "20",                            // Constant Rate Factor - balances quality and compression (lower = better quality)
		"sc_threshold":         "0",                             // Disable scene change detection for keyframes (forces regular keyframes)
		"g":                    "48",                            // GOP size: one keyframe every 48 frames (assuming ~2s GOP for 24fps)
		"keyint_min":           "48",                            // Minimum interval between keyframes (same as GOP)
		"b:v":                  q.Bitrate,                       // Target video bitrate for this quality level
		"maxrate":              q.Maxrate,                       // Maximum allowed video bitrate
		"bufsize":              q.Bufsize,                       // Buffer size for rate control
		"c:a":                  "aac",                           // Use AAC audio codec
		"ar":                   "48000",                         // Audio sampling rate (48kHz)
		"b:a":                  "128k",                          // Audio bitrate
		"hls_list_size":        "0",                             // Ensure the entire playlist is written (not a sliding window)
		"hls_time":             "6",                             // Duration of each segment in seconds
		"hls_playlist_type":    "vod",                           // Indicate this is a video-on-demand playlist
		"start_number":         "1",                             // Start segment numbering from 1
		"hls_segment_filename": segmentPath,                     // Pattern for naming the TS segment files
		"hls_flags":            "round_durations+split_by_time", // Round segment durations and split strictly by time
		"hls_allow_cache":      "1",                             // Allow caching of HLS segments
		"vf":                   filters[0],                      // Video filter (e.g., scaling)
		"s":                    filters[1],                      // Output resolution (explicit)

	}
}

func createTempFileDir(id string, reader io.Reader) (*os.File, string, string, error) {
	tempDir := filepath.Join(os.TempDir(), "vod_"+id)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up entire temp directory

	// Save uploaded video to temporary file
	uploadedVideoPath := filepath.Join(tempDir, "uploaded_video.mp4")
	uploadedFile, err := os.Create(uploadedVideoPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create temp file for uploaded video: %w", err)
	}
	if _, err := io.Copy(uploadedFile, reader); err != nil {
		uploadedFile.Close()
		return nil, "", "", fmt.Errorf("failed to save uploaded video: %w", err)
	}
	return uploadedFile, tempDir, uploadedVideoPath, nil
}

func (s *FFmpegService) GetSnapshot(id string, input entity.Path) (*os.File, error) {
	// Create temp directory for snapshot
	tempDir := filepath.Join(os.TempDir(), "vod_snapshot_"+id)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "snapshot.jpg")
	timestamp := "00:00:05" // default to 5 seconds into the video

	cmd := exec.Command("ffmpeg",
		"-i", input.String(),
		"-ss", timestamp,
		"-vframes", "1",
		"-q:v", "2",
		outputPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w, output: %s", err, string(output))
	}

	snapshotFile, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %w", err)
	}
	return snapshotFile, nil
}

func (uc *VideoUseCase) concatenateVideos(id string, reader io.Reader) (*os.File, error) {
	// Create temporary directory for processing
	uploadedFile, tempDir, uploadedVideoPath, err := createTempFileDir(id, reader)
	if err != nil {
		return nil, err
	}
	uploadedFile.Close()

	// Check if trademark video exists
	trademarkVideoPath := filepath.Join("test_files", "netflix-intro.mp4")
	if _, err := os.Stat(trademarkVideoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("trademark video not found at %s", trademarkVideoPath)
	}

	const targetW, targetH = 1280, 720

	/*WHAT I LEARNT*/
	// Normalize both inputs to the same resolution/SAR/fps before concatenating.
	// concat requires matching stream parameters across all inputs, which is
	// why mixing a 1920x800 intro with a 640x360 upload was failing.

	filterComplex := fmt.Sprintf(
		"[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30[v0];"+
			"[1:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30[v1];"+
			"[0:a]aresample=44100[a0];"+
			"[1:a]aresample=44100[a1];"+
			"[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
		targetW, targetH, targetW, targetH,
		targetW, targetH, targetW, targetH,
	)

	// Concatenate videos using FFmpeg filter_complex
	concatenatedPath := filepath.Join(tempDir, "concatenated.mp4")
	cmd := exec.Command("ffmpeg",
		"-i", trademarkVideoPath,
		"-i", uploadedVideoPath,
		"-filter_complex", filterComplex,
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "h264",
		"-c:a", "aac",
		concatenatedPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to concatenate videos: %w, output: %s", err, string(output))
	}

	// Read concatenated video and save to storage
	concatenatedFile, err := os.Open(concatenatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open concatenated video: %w", err)
	}
	return concatenatedFile, nil
}

func (s *FFmpegService) extractAudio(input entity.Path, id string) (*os.File, error) {
	tempDir := filepath.Join(os.TempDir(), "vod_audio_"+id)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	output := filepath.Join(tempDir, "audio.wav")
	cmd := ffmpeg_go.Input(input.String()).Output(output, ffmpeg_go.KwArgs{
		"vn": "",    // Disable video
		"ac": 1,     // Mono
		"ar": 16000, // 16 kHz
	})
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract audio: %w, output: %s", err, output)
	}

	outputFile, err := os.Open(output)

	if err != nil {
		return nil, fmt.Errorf("failed to open extracted audio file: %w", err)
	}
	return outputFile, nil
}

func (uc *VideoUseCase) ProcessAndSave(filename string, reader io.Reader) error {
	id := uuid.New().String()
	concatenatedFile, err := uc.concatenateVideos(id, reader)

	if err != nil {
		return fmt.Errorf("failed to concatenate videos: %w", err)
	}
	fmt.Println("Concetenated File name:", concatenatedFile.Name())

	savedDetails, err := uc.Storage.Save(concatenatedFile, "videos", id, "concatenated.mp4")
	if err != nil {
		return fmt.Errorf("failed to save video: %w", err)
	}
	fmt.Println("Video saved with details:", savedDetails)

	// Get snapshot from the saved file (fully written to disk)
	snapshotFile, err := uc.FFmpeg.GetSnapshot(id, savedDetails)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	savedSnapshot, err := uc.Storage.Save(snapshotFile, "videos", id, "snapshot.jpg")
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	fmt.Printf("Snapshot saved: %s\n", savedSnapshot)

	outputAudio, err := uc.FFmpeg.extractAudio(entity.StringPathToPath(concatenatedFile.Name()), id)
	if err != nil {
		return fmt.Errorf("failed to extract audio: %w", err)
	}
	audioFile, err := uc.Storage.Save(outputAudio, "videos", id, "audio", id+".wav")
	if err != nil {
		return fmt.Errorf("failed to save audio: %w", err)
	}
	fmt.Printf("Audio saved: %s\n", audioFile)

	audioPathReader, err := os.Open("./uploads/videos/" + id + "/audio/" + id + ".wav")
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioPathReader.Close()

	transcript, err := uc.ExtractTextFromAudio(id, audioPathReader)
	if err != nil {
		return err
	}
	fmt.Println("Extracted Transcript:", transcript)
	transcriptFile, err := uc.Storage.Save(strings.NewReader(transcript), "videos", id, "transcript.txt")
	if err != nil {
		return fmt.Errorf("failed to save transcript: %w", err)
	}
	fmt.Printf("Transcript saved: %s\n", transcriptFile)

	videoDetails, err := uc.FFmpeg.GetVideoDetails(savedDetails)
	if err != nil {
		return fmt.Errorf("failed to get video details: %w", err)
	}
	fmt.Printf("Video Details: %+v\n", videoDetails)

	if videoDetails == nil {
		return fmt.Errorf("no video details available")
	}
	/*isPortrait := videoDetails.IsPortrait()
	if err := uc.ffmpeg.Transcode(savedDetails, isPortrait); err != nil {
		return fmt.Errorf("failed to transcode video: %w", err)
	}

	masterURL, err := UploadHLSToS3("./uploads/videos/"+id, id, "vod2")
	if err != nil {
		fmt.Println("Failed to upload HLS to S3:", err)
		return fmt.Errorf("failed to upload HLS to S3: %w", err)
	}
	fmt.Println("HLS uploaded to S3 successfully. Master URL:", masterURL)*/

	// Pass nil for videoDetails to avoid type mismatch with jobs.EnqueueTranscodeJob
	// which expects a different VideoData type. The job can probe details itself if needed.
	jobs.EnqueueTranscodeJob(savedDetails, id)
	return nil
}

func (uc *VideoUseCase) ExtractTextFromAudio(id string, reader io.Reader) (string, error) {
	ASSEMBLYAI_API := os.Getenv("ASSEMBLYAI_API_KEY")
	if ASSEMBLYAI_API == "" {
		return "", fmt.Errorf("ASSEMBLYAI_API_KEY is not set in environment variables")
	}
	client := assemblyai.NewClient(ASSEMBLYAI_API)

	transcript, err := client.Transcripts.TranscribeFromReader(
		context.Background(),
		reader,
		nil,
	)
	if err != nil {
		return "", err
	}

	fmt.Println(*transcript.Text)
	return *transcript.Text, nil
}

func (s *FFmpegService) generateMasterPlaylist(outputDir entity.Path) error {
	masterFilePath := filepath.Join(outputDir.String(), "master.m3u8")

	masterFile, err := os.Create(masterFilePath)
	if err != nil {
		return err
	}
	defer masterFile.Close()

	writer := bufio.NewWriter(masterFile)
	defer writer.Flush()

	if _, err = writer.WriteString("#EXTM3U\n"); err != nil {
		return err
	}

	for _, q := range s.VideoQualities {
		bandwidth := extractBandwidth(q.Bitrate)
		line := fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\nnormal_hls/%s/index.m3u8\n", bandwidth, q.LandScape(), q.Name)
		if _, err = writer.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

func extractBandwidth(bitrate string) int {
	bitrate = strings.TrimSuffix(bitrate, "k")
	kbps, err := strconv.Atoi(bitrate)
	if err != nil {
		return 0
	}
	return kbps * 1000
}
