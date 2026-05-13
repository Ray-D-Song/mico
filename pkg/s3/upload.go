package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadFile uploads a local file to the given S3 bucket and key.
func UploadFile(ctx context.Context, bucket, key, filePath string) error {
	client := GetClient()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer f.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}
	return nil
}

type ObjectInfo struct {
	Key          string
	LastModified time.Time
}

// ListObjects lists objects under the given prefix in the specified bucket.
// Returns up to maxKeys objects.
func ListObjects(ctx context.Context, bucket, prefix string, maxKeys int32) ([]ObjectInfo, error) {
	client := GetClient()

	var objects []ObjectInfo
	var continuationToken *string

	for {
		resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &bucket,
			Prefix:            &prefix,
			MaxKeys:           &maxKeys,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in %s: %w", bucket, err)
		}

		for _, obj := range resp.Contents {
			if obj.Key != nil {
				lm := time.Time{}
				if obj.LastModified != nil {
					lm = *obj.LastModified
				}
				objects = append(objects, ObjectInfo{Key: *obj.Key, LastModified: lm})
			}
		}

		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		continuationToken = resp.NextContinuationToken
	}

	return objects, nil
}

// DeleteObject deletes a single object from the given bucket.
func DeleteObject(ctx context.Context, bucket, key string) error {
	client := GetClient()

	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}
	return nil
}

// DownloadFile downloads an object from S3 to a local file.
func DownloadFile(ctx context.Context, bucket, key, destPath string) error {
	client := GetClient()

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", key, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file for download: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write downloaded file: %w", err)
	}
	return nil
}
