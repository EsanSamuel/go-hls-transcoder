package jobs

import (
	"log"

	"github.com/EsanSamuel/go-hls-transcoder/internal/entity"
	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"
)

var redisPool *redis.Pool = &redis.Pool{
	MaxIdle:   3,
	MaxActive: 10,
	Wait:      true,
	Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", "localhost:6379")
	},
}

var enqueuer *work.Enqueuer = work.NewEnqueuer("vod", redisPool)

func EnqueueTranscodeJob(filePath entity.Path, id string) {
	_, err := enqueuer.Enqueue("process_transcoder", work.Q{"file_path": filePath.String(), "id": id})
	if err != nil {
		log.Fatal(err)
	}
}
