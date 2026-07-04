# VOD - Video On Demand Service

A Go-based video processing service that accepts uploaded videos, concatenates them with a trademark clip, extracts a thumbnail and audio, runs transcription, and generates HLS outputs for adaptive streaming.

## What the service does

When a video is uploaded, the server currently performs the following workflow:

1. Saves the uploaded file to the local uploads directory.
2. Concatenates the uploaded video with a trademark video from test_files/trademark.mp4.
3. Extracts a snapshot at 5 seconds.
4. Extracts audio from the concatenated video.
5. Sends the audio to AssemblyAI for transcription.
6. Generates HLS playlists and segment files for multiple quality levels.
7. Attempts to upload the generated HLS output to S3-compatible storage if the required AWS settings are present.

## Requirements

- Go 1.26 or newer
- FFmpeg installed and available in your PATH
- FFprobe available via FFmpeg
- A trademark video placed at test_files/trademark.mp4

## Installation

1. Clone the repository.
2. Install Go dependencies:

```bash
go mod download
```

3. Create a `.env` file in the project root with the following values:

```env
PORT=8080
ASSEMBLYAI_API_KEY=your_assemblyai_key
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
```

4. Make sure the trademark file exists:

```bash
mkdir -p test_files
# place your trademark.mp4 file at test_files/trademark.mp4
```

## Running the server

```bash
go run ./cmd/server
```

The server starts on port `8080` by default, or on the value provided in `PORT`.

## API

### Health check

```bash
curl http://localhost:8080/health
```

Example response:

```json
{
  "status": "ok"
}
```

### Upload a video

```bash
curl -X POST http://localhost:8080/upload \
  -F "video=@/path/to/video.mp4"
```

Example response:

```json
{
  "message": "Video uploaded successfully!",
  "filename": "video.mp4"
}
```

## Output locations

Processed assets are written under the local uploads directory:

```text
uploads/
└── videos/
    └── {video_id}/
        ├── concatenated.mp4
        ├── snapshot.jpg
        ├── audio/
        │   └── {video_id}.wav
        └── normal_hls/
            ├── master.m3u8
            ├── 1080p/
            ├── 720p/
            ├── 480p/
            ├── 360p/
            ├── 240p/
            └── 144p/
```

## Notes

- The upload endpoint requires a multipart form field named `video`.
- Transcription depends on `ASSEMBLYAI_API_KEY` being set.
- S3 upload is attempted when AWS credentials and bucket configuration are available; otherwise the process may fail after HLS generation.
- The current implementation uses local filesystem storage for the generated video artifacts.

## Project structure

```text
cmd/server/main.go      # HTTP server and processing pipeline
entity/entity.go        # Path helpers and shared models
test_files/             # Trademark media used during concatenation
uploads/                # Local output directory for generated assets
```

## Troubleshooting

### FFmpeg Not Found
Ensure FFmpeg is installed and in your system PATH:
```bash
ffmpeg -version
ffprobe -version
```

### S3 Upload Failures
- Verify AWS credentials in `.env`
- Check S3 endpoint URL (default: `t3.storage.dev`)
- Ensure bucket exists and is accessible

### "moov atom not found" Error
This occurs when FFmpeg tries to read an incomplete video file. The solution is to ensure the file is fully written to disk before reading it.

### Out of Disk Space
Monitor the `uploads/` directory and clean up old videos periodically:
```bash
# Remove videos older than 7 days
find uploads/videos -type d -ctime +7 -exec rm -rf {} \;
```

## License

MIT

## Author

EsanSamuel

## Support

For issues and questions, please refer to the main documentation or contact the development team.
