package utils

import (
	"os"
	"testing"
)

func TestCleanupRemovesAllWorkDirs(t *testing.T) {
	Cleanup()
	t.Cleanup(Cleanup)

	first := CreateWorkDir("mico-test")
	second := CreateWorkDir("mico-test")
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("first work directory was not created: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second work directory was not created: %v", err)
	}

	Cleanup()
	for _, dir := range []string{first, second} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("work directory still exists after cleanup: %s", dir)
		}
	}

	Cleanup()
}

func TestRemoveWorkDirIsIdempotent(t *testing.T) {
	Cleanup()
	t.Cleanup(Cleanup)

	dir := CreateWorkDir("mico-test")
	RemoveWorkDir(dir)
	RemoveWorkDir(dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("work directory still exists after removal: %s", dir)
	}
}
