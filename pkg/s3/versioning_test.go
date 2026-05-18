package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

func TestBucketVersioningEnabledFromStatus(t *testing.T) {
	tests := []struct {
		name   string
		status types.BucketVersioningStatus
		want   bool
	}{
		{name: "enabled", status: types.BucketVersioningStatusEnabled, want: true},
		{name: "suspended", status: types.BucketVersioningStatusSuspended, want: true},
		{name: "disabled", status: types.BucketVersioningStatus("Disabled"), want: false},
		{name: "empty", status: types.BucketVersioningStatus(""), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, bucketVersioningEnabledFromStatus(tt.status))
		})
	}
}

func TestBucketVersioningStateGetterSetter(t *testing.T) {
	t.Cleanup(func() {
		SetBucketVersioningEnabled(false)
	})

	SetBucketVersioningEnabled(true)
	require.True(t, IsBucketVersioningEnabled())

	SetBucketVersioningEnabled(false)
	require.False(t, IsBucketVersioningEnabled())
}
