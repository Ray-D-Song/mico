package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
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
	Size         int64
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

// ObjectVersionInfo describes a single version or delete marker for an object.
type ObjectVersionInfo struct {
	Key          string
	VersionID    string
	IsDeleteMark bool
	LastModified time.Time
	Size         int64
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
				sz := int64(0)
				if obj.Size != nil {
					sz = *obj.Size
				}
				objects = append(objects, ObjectInfo{Key: *obj.Key, LastModified: lm, Size: sz})
			}
		}

		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		continuationToken = resp.NextContinuationToken
	}

	return objects, nil
}

// DeleteObjectVersion permanently deletes a specific object version or delete marker.
func DeleteObjectVersion(ctx context.Context, bucket, key, versionID string) error {
	client := GetClient()

	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket:    &bucket,
		Key:       &key,
		VersionId: &versionID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete %s version %s: %w", key, versionID, err)
	}
	return nil
}

// ListObjectVersions lists all versions and delete markers under the given prefix.
func ListObjectVersions(ctx context.Context, bucket, prefix string, maxKeys int32) ([]ObjectVersionInfo, error) {
	client := GetClient()

	var versions []ObjectVersionInfo
	var keyMarker *string
	var versionIDMarker *string

	for {
		resp, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket:          &bucket,
			Prefix:          &prefix,
			MaxKeys:         &maxKeys,
			KeyMarker:       keyMarker,
			VersionIdMarker: versionIDMarker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list object versions in %s: %w", bucket, err)
		}

		appendVersion := func(key *string, versionID *string, lastModified *time.Time, size *int64, isDeleteMarker bool) {
			if key == nil || versionID == nil {
				return
			}
			lm := time.Time{}
			if lastModified != nil {
				lm = *lastModified
			}
			sz := int64(0)
			if size != nil {
				sz = *size
			}
			versions = append(versions, ObjectVersionInfo{
				Key:          *key,
				VersionID:    *versionID,
				IsDeleteMark: isDeleteMarker,
				LastModified: lm,
				Size:         sz,
			})
		}

		for _, v := range resp.Versions {
			appendVersion(v.Key, v.VersionId, v.LastModified, v.Size, false)
		}
		for _, dm := range resp.DeleteMarkers {
			appendVersion(dm.Key, dm.VersionId, dm.LastModified, nil, true)
		}

		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		keyMarker = resp.NextKeyMarker
		versionIDMarker = resp.NextVersionIdMarker
	}

	return versions, nil
}

// DeleteAllObjectVersions deletes every version and delete marker for a given key.
func DeleteAllObjectVersions(ctx context.Context, bucket, key string) error {
	versions, err := ListObjectVersions(ctx, bucket, key, 1000)
	if err != nil {
		return err
	}

	filtered := make([]ObjectVersionInfo, 0, len(versions))
	for _, version := range versions {
		if version.Key == key {
			filtered = append(filtered, version)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].LastModified.Equal(filtered[j].LastModified) {
			return filtered[i].VersionID > filtered[j].VersionID
		}
		return filtered[i].LastModified.After(filtered[j].LastModified)
	})

	for _, version := range filtered {
		if err := DeleteObjectVersion(ctx, bucket, version.Key, version.VersionID); err != nil {
			return err
		}
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
