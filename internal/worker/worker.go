package worker

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/EsanSamuel/go-hls-transcoder/internal/config"
	"github.com/EsanSamuel/go-hls-transcoder/internal/entity"
	"github.com/EsanSamuel/go-hls-transcoder/internal/video"
	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"
)

type Context struct {
	id         string
	isPortrait bool
	filePath   entity.Path
	uc         video.VideoUseCase
	v          *video.VideoData
}

var redisPool = &redis.Pool{
	MaxActive: 5,
	MaxIdle:   5,
	Wait:      true,
	Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", ":6379")
	},
}

func VodWorker() {
	pool := work.NewWorkerPool(Context{}, 10, "vod", redisPool)

	pool.Middleware((*Context).Log)
	pool.Middleware((*Context).Find)

	pool.Job("process_transcoder", (*Context).ProcessTranscoder)

	//pool.JobWithOptions("export", work.JobOptions{Priority: 10, MaxFails: 1}, (*Context).Export)

	pool.Start()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	<-signalChan

	// Stop the pool
	pool.Stop()
}

func (c *Context) Log(job *work.Job, next work.NextMiddlewareFunc) error {
	fmt.Println("Starting job: ", job.Name)
	return next()
}

func (c *Context) Find(job *work.Job, next work.NextMiddlewareFunc) error {
	// If there's a customer_id param, set it in the context for future middleware and handlers to use.
	if _, ok := job.Args["id"]; ok {
		c.id = job.ArgString("id")
		if err := job.ArgError(); err != nil {
			return err
		}
	}

	return next()
}

func (c *Context) IsPortrait() bool {
	for _, stream := range c.v.Streams {
		if stream.CodecType == "video" {
			return stream.Height > stream.Width
		}
	}
	return false
}

func (c *Context) ProcessTranscoder(job *work.Job) error {
	// Extract arguments:
	filePath := entity.NewPath(job.ArgString("file_path"))
	id := job.ArgString("id")
	if err := job.ArgError(); err != nil {
		return err
	}
	isPortrait := c.IsPortrait()
	if err := c.uc.FFmpeg.Transcode(filePath, isPortrait); err != nil {
		return fmt.Errorf("failed to transcode video: %w", err)
	}

	masterURL, err := config.UploadHLSToS3("./uploads/videos/"+id, id, "vod2")
	if err != nil {
		fmt.Println("Failed to upload HLS to S3:", err)
		return fmt.Errorf("failed to upload HLS to S3: %w", err)
	}
	fmt.Println("HLS uploaded to S3 successfully. Master URL:", masterURL)

	return nil
}
