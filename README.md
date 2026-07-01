# VOD - Video On Demand Service

A Go-based video processing and streaming service that handles video upload, concatenation, thumbnail extraction, and HLS transcoding for multi-quality adaptive streaming.

## Features

- **Video Concatenation**: Automatically concatenate uploaded videos with a trademark video
- **Snapshot Extraction**: Extract thumbnail images from videos at customizable timestamps
- **Multi-Quality HLS Transcoding**: Convert videos to 6 quality levels (144p to 1080p)
- **AWS S3 Integration**: Upload HLS artifacts to S3-compatible storage
- **REST API**: Simple HTTP endpoints for video management
- **Clean Architecture**: Dependency injection with Go interfaces for maintainability

## Architecture

The project uses **Clean Architecture** principles with clear separation of concerns:

```
VideoController (HTTP Layer)
    ↓
VideoUseCase (Business Logic)
    ↓
VideoStorage Interface  +  FFmpeg Interface
    ↓                           ↓
FileSystemStorage         FFmpegService
(File I/O)               (Video Processing)
```

### Key Components

- **VideoStorage Interface**: Abstracts video file storage operations
- **FFmpeg Interface**: Abstracts video processing operations (transcoding, probing, snapshots)
- **VideoUseCase**: Orchestrates the video processing pipeline
- **VideoController**: HTTP request handlers

## Supported Quality Levels

| Quality | Resolution | Bitrate | Max Rate | Buffer Size |
|---------|-----------|---------|----------|------------|
| 1080p   | 1920x1080 | 4500k   | 4700k    | 6000k      |
| 720p    | 1280x720  | 2500k   | 2675k    | 3750k      |
| 480p    | 854x480   | 1000k   | 1075k    | 1500k      |
| 360p    | 640x360   | 600k    | 650k     | 900k       |
| 240p    | 426x240   | 400k    | 450k     | 600k       |
| 144p    | 256x144   | 250k    | 275k     | 400k       |

## Prerequisites

- Go 1.16+
- FFmpeg with libmp4box support
- FFprobe (included with FFmpeg)

## Installation

### 1. Clone the repository
```bash
cd c:\Users\user\Desktop\VOD
```

### 2. Install dependencies
```bash
go mod download
```

### 3. Set environment variables
Create a `.env` file in the project root:
```env
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
PORT=8080
```

### 4. Prepare test files
Place your trademark video at:
```
test_files/trademark.mp4
```

## Usage

### Start the Server

```bash
go run cmd/server/main.go
```

The server will start on `http://localhost:8080` (or the port specified in `PORT` environment variable).

### API Endpoints

#### Health Check
```bash
GET /health
```

Response:
```json
{
  "status": "ok"
}
```

#### Upload Video
```bash
POST /upload
Content-Type: multipart/form-data

Form field: "video" (binary video file)
```

Response:
```json
{
  "message": "Video uploaded successfully!",
  "filename": "example.mp4"
}
```

## Processing Pipeline

When a video is uploaded, the following steps occur:

1. **Concatenation**: Upload → Trademark concatenation using FFmpeg filter_complex
2. **Storage**: Save concatenated video to permanent storage
3. **Snapshot**: Extract thumbnail from saved video (5 seconds mark)
4. **Video Details**: Probe video to extract metadata (resolution, codec, duration)
5. **Transcoding** *(commented out)*: Convert to 6 quality levels with HLS segments
6. **S3 Upload** *(commented out)*: Upload HLS artifacts to S3

## File Structure

```
VOD/
├── cmd/
│   └── server/
│       ├── main.go              # Main application code
│       └── uploads/             # Local storage for videos
│           └── videos/
│               └── {video_id}/
│                   ├── concatenated.mp4
│                   ├── snapshot.jpg
│                   └── normal_hls/
│                       ├── 1080p/
│                       ├── 720p/
│                       ├── ...
│                       └── 144p/
├── entity/
│   └── entity.go                # Data models
├── test_files/
│   └── trademark.mp4            # Trademark video for concatenation
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## Storage Paths

### Local File System
```
uploads/
└── videos/
    └── {UUID}/
        ├── concatenated.mp4           # Original concatenated video
        ├── snapshot.jpg               # Video thumbnail
        └── normal_hls/
            ├── master.m3u8            # Main HLS playlist
            ├── 1080p/
            │   ├── index.m3u8
            │   ├── 001.ts
            │   └── ...
            ├── 720p/
            └── ...
```

### AWS S3 (if enabled)
```
s3://bucket/
└── videos/
    └── {video_id}/
        ├── normal_hls/
        │   ├── master.m3u8
        │   ├── 1080p/
        │   ├── 720p/
        │   └── ...
```

## Dependencies

### Core Dependencies
- **gin-gonic/gin**: HTTP web framework
- **aws-sdk-go-v2**: AWS SDK for S3 operations
- **ffmpeg-go**: Go wrapper for FFmpeg
- **google/uuid**: UUID generation
- **joho/godotenv**: Environment variable loading

### External Tools
- **FFmpeg**: Video processing and encoding
- **FFprobe**: Video probing and metadata extraction

## Error Handling

The application uses error wrapping with `%w` to preserve error chains:

```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

This allows callers to inspect root causes using `errors.Is()` and `errors.As()`.

## Clean Architecture Principles Used

### Interfaces
Dependency inversion through interfaces:
```go
type VideoStorage interface {
    Save(reader io.Reader, path ...string) (entity.Path, error)
    Open(path ...string) (io.ReadCloser, error)
    GetPath(path ...string) (entity.Path, error)
}

type FFmpeg interface {
    Transcode(input entity.Path, isPortrait bool) error
    GetVideoDetails(path entity.Path) (*VideoData, error)
    GetSnapshot(id string, input entity.Path) (*os.File, error)
}
```

### Dependency Injection
Dependencies are injected into the VideoUseCase:
```go
type VideoUseCase struct {
    storage VideoStorage
    ffmpeg  FFmpeg
}
```

### Single Responsibility
Each component has a single, well-defined responsibility:
- Controllers: HTTP request/response handling
- UseCases: Business logic orchestration
- Services: Implementation of specific operations

## Performance Considerations

- **Temporary Files**: Large video files are processed through temporary directories to avoid memory exhaustion
- **HLS Segments**: 6-second segments provide good balance between seeking accuracy and file count
- **Bitrate Control**: CBR with max rate limiting prevents bandwidth spikes
- **Parallel Quality Encoding**: Multiple quality levels can be encoded (currently sequential)

## Future Enhancements

- [ ] Enable parallel transcoding for multiple quality levels
- [ ] Re-enable S3 upload functionality
- [ ] Add video format validation
- [ ] Implement progress tracking for long-running operations
- [ ] Add database integration for video metadata
- [ ] Support for custom video layouts (watermarks, overlays)
- [ ] Implement video trimming/clipping
- [ ] Add subtitle/caption support

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
