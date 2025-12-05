//go:build s3
// +build s3

package backup

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage interface using AWS S3
type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Storage creates a new S3 storage
func NewS3Storage(client *s3.Client, bucket, prefix string) *S3Storage {
	return &S3Storage{
		client: client,
		bucket: bucket,
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// key returns the full S3 key for a given name
func (s *S3Storage) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", s.prefix, name)
}

// Save saves data to S3
func (s *S3Storage) Save(ctx context.Context, name string, data io.Reader) error {
	key := s.key(name)

	// Read all data
	body, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Upload to S3
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(body)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// Load loads data from S3
func (s *S3Storage) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	key := s.key(name)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return result.Body, nil
}

// List lists all objects with the given prefix
func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	keyPrefix := s.key(prefix)

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(keyPrefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects from S3: %w", err)
	}

	var names []string
	for _, obj := range result.Contents {
		// Remove prefix from key
		key := *obj.Key
		if s.prefix != "" {
			key = strings.TrimPrefix(key, s.prefix+"/")
		}
		names = append(names, key)
	}

	return names, nil
}

// Delete deletes an object from S3
func (s *S3Storage) Delete(ctx context.Context, name string) error {
	key := s.key(name)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}

