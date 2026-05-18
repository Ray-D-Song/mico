package s3

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	bucketVersioningMu      sync.RWMutex
	bucketVersioningEnabled bool
)

// IsBucketVersioningEnabled reports the last detected bucket versioning state.
func IsBucketVersioningEnabled() bool {
	bucketVersioningMu.RLock()
	defer bucketVersioningMu.RUnlock()
	return bucketVersioningEnabled
}

// SetBucketVersioningEnabled overrides the cached bucket versioning state.
func SetBucketVersioningEnabled(enabled bool) {
	bucketVersioningMu.Lock()
	bucketVersioningEnabled = enabled
	bucketVersioningMu.Unlock()
}

func bucketVersioningEnabledFromStatus(status types.BucketVersioningStatus) bool {
	switch status {
	case types.BucketVersioningStatusEnabled, types.BucketVersioningStatusSuspended:
		return true
	default:
		return false
	}
}

// DetectBucketVersioning probes the bucket versioning state and caches the result.
// On any probe failure, the cached state is set to false and the error is returned.
func DetectBucketVersioning(ctx context.Context, bucket string) error {
	client := GetClient()

	resp, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: &bucket,
	})
	if err != nil {
		SetBucketVersioningEnabled(false)
		return fmt.Errorf("failed to detect bucket versioning for %s: %w", bucket, err)
	}

	SetBucketVersioningEnabled(bucketVersioningEnabledFromStatus(resp.Status))
	return nil
}
