package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/EsanSamuel/go-hls-transcoder/internal/entity"
)

type VideoStorage interface {
	Save(reader io.Reader, path ...string) (entity.Path, error) // Save video data to storage
	Open(path ...string) (io.ReadCloser, error)                 // Open video file for reading
	GetPath(path ...string) (entity.Path, error)                // Get absolute path of stored video file
}

type FileSystemStorage struct {
	BaseDir entity.Path
}

func NewFileSystemStorage(baseDir string) *FileSystemStorage {
	if baseDir == "" {
		baseDir = "uploads"
	}
	return &FileSystemStorage{BaseDir: entity.NewPath(baseDir)}
}

func (s *FileSystemStorage) Save(reader io.Reader, path ...string) (entity.Path, error) {
	fullPath := s.BaseDir.Join(path...).String()

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return entity.Path{}, fmt.Errorf("failed to create directories: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return entity.Path{}, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(fullPath) // clean up partial write
		return entity.Path{}, fmt.Errorf("failed to save video data: %w", err)
	}

	return entity.StringPathToPath(fullPath), nil
}

// Open opens a file for reading at the given path (relative to BasePath).
// Returns a ReadCloser for the file, or an error if the file cannot be opened.
func (s *FileSystemStorage) Open(path ...string) (io.ReadCloser, error) {
	fullPath := s.BaseDir.Join(path...).String()
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open video file: %w", err)
	}
	return file, nil
}

// GetPath returns the absolute Path for the given relative path, if the file exists.
// Returns an error if the file does not exist.
func (s *FileSystemStorage) GetPath(path ...string) (entity.Path, error) {
	fullPath := s.BaseDir.Join(path...).String()

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return entity.Path{}, fmt.Errorf("file does not exist: %s", fullPath)
	}

	return entity.StringPathToPath(fullPath), nil
}
