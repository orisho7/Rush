package rush

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

var s3Client *s3.Client
var bucketName string

func SetupS3() {
	_ = godotenv.Load()

	endpoint := os.Getenv("RUSH_S3_ENDPOINT")

	bucketName = os.Getenv("RUSH_S3_BUCKET")
	if bucketName == "" {
		bucketName = "rush-cache"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load configuration, %v", err)
	}

	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
		o.Region = os.Getenv("AWS_REGION")
		if o.Region == "" {
			o.Region = "auto"
		}
	})
}
func StoreinS3(hash string) error {
	defer wg.Done()
	// 1 - Compressing file with Zstd
	zipStart := time.Now()
	checksum, err := compress(hash)
	if err != nil {
		UpdateS3Status(hash, "failed")
		return err
	}
	fmt.Printf("✓ Local Compression: %v\n", time.Since(zipStart))

	// 2 - Uploading to S3
	archivePath := filepath.Join(".rush-cache", hash+".tar.zst")
	defer os.Remove(archivePath) // Clean up disk after upload attempt

	uploadStart := time.Now()
	err = UploadToS3(archivePath, hash+".tar.zst", checksum)
	if err != nil {
		fmt.Printf("Upload failed: %v\n", err)
		UpdateS3Status(hash, "failed")
	} else {
		fmt.Printf("✓ Cloud Upload: %v\n", time.Since(uploadStart))
		UpdateS3Status(hash, "ready") // Signal to other machines that the hash is ready
	}
	return err
}

type CacheStatus struct {
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

func UpdateS3Status(hash string, state string) error {
	statusBytes, _ := json.Marshal(CacheStatus{
		State:     state,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(hash + "/status.json"),
		Body:   strings.NewReader(string(statusBytes)),
	})
	return err
}

func CheckS3Status(hash string) (*CacheStatus, error) {
	out, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(hash + "/status.json"),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	var status CacheStatus
	if err := json.NewDecoder(out.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

func UploadToS3(localPath string, s3Key string, checksum string) error {
	fmt.Println("Uploading to S3 (Stable Parallel)...")
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10MB per part
		u.Concurrency = 20            // 20 concurrent goroutines
	})

	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(s3Key),
		Body:     file,
		Metadata: map[string]string{"checksum": checksum},
	})
	return err
}

func DownloadFromS3(filename string) (*s3.GetObjectOutput, string, error) {
	fmt.Println("Downloading from S3 (Standard Stream)...")
	out, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filename),
	})
	if err != nil {
		return nil, "", err
	}
	return out, out.Metadata["checksum"], nil
}
