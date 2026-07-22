package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func UploadHLSToS3(localDir, videoID, bucket string) (string, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return "", fmt.Errorf("missing S3 credentials: access=%v secret=%v", accessKey != "", secretKey != "")
	}
	fmt.Println("Uploading to S3...")

	cfg, _ := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		)))

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("https://t3.storage.dev")
	})

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Preserve folder structure in S3
		relativePath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		relativePath = filepath.ToSlash(relativePath)

		s3Key := "videos/" + videoID + "/" + relativePath

		// Set correct content type
		contentType := "video/MP2T" // for .ts files
		if strings.HasSuffix(path, ".m3u8") {
			contentType = "application/x-mpegURL"
		}

		fmt.Println("Uploading:", s3Key)

		_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(s3Key),
			Body:        file,
			ContentType: aws.String(contentType),
		})
		return err
	})

	if err != nil {
		return "", err
	}

	// Return the master playlist URL
	masterURL := "https://" + bucket + ".t3.storage.dev/videos/" + videoID + "/master.m3u8"
	return masterURL, nil
}
