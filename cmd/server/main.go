package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/EsanSamuel/go-hls-transcoder/internal/rag"
	"github.com/EsanSamuel/go-hls-transcoder/internal/storage"
	"github.com/EsanSamuel/go-hls-transcoder/internal/video"
	"github.com/EsanSamuel/go-hls-transcoder/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type VideoController struct {
	videoUseCase *video.VideoUseCase // Use case for handling video-related operations
}

type AskAIRequest struct {
	VideoID  string `json:"video_id"`
	Question string `json:"question"`
}

type AskAIResponse struct {
	Answer string  `json:"answer"`
	Score  float32 `json:"score"`
	Prompt string  `json:"prompt"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (v *VideoController) UploadVideoHandler(c *gin.Context) {
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get video file"})
		return
	}
	defer file.Close()

	filename := header.Filename
	if err := v.videoUseCase.ProcessAndSave(filename, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video uploaded successfully!", "filename": filename})
}

func (v *VideoController) AskAIWebSocketHandler(c *gin.Context) {
	videoID := c.Param("videoID")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	transcriptPath := fmt.Sprintf("./uploads/videos/%s/transcript.txt", videoID)
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "failed to load transcript"})
		return
	}
	chunks := rag.ChunkText(string(data))

	for {
		var req AskAIRequest
		if err := conn.ReadJSON(&req); err != nil {
			break // client disconnected or sent bad data
		}

		score, answer, prompt, err := rag.ProcessChunks(chunks, req.Question)
		if err != nil {
			conn.WriteJSON(gin.H{"error": "AI request failed"})
			continue
		}
		fmt.Println(AskAIResponse{Answer: answer, Score: score, Prompt: prompt})

		conn.WriteJSON(AskAIResponse{Answer: answer, Score: score, Prompt: prompt})
	}
}

func main() {
	r := gin.Default()

	key := godotenv.Load(".env")
	if key != nil {
		fmt.Println("Error loading .env file:", key)
	}
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	fmt.Println("AccessKey:", accessKey)
	fmt.Println("SecretKey:", secretKey)

	storage := storage.NewFileSystemStorage("uploads")
	if err := os.MkdirAll(storage.BaseDir.String(), 0o755); err != nil {
		log.Fatalf("failed to create uploads directory: %v", err)
	}

	ffmpegService := &video.FFmpegService{VideoQualities: video.VideoQualities}
	videoUseCase := &video.VideoUseCase{Storage: storage, FFmpeg: ffmpegService}
	controller := &VideoController{videoUseCase: videoUseCase}

	go worker.VodWorker()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/upload", controller.UploadVideoHandler)
	r.GET("ws/ask-AI/:videoID", controller.AskAIWebSocketHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
