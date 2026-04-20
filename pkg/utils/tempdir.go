package utils

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var (
	tempDir    string
	cleanupFn func()
)

func SetTempDir(dir string) {
	tempDir = dir
}

func GetTempDir() string {
	return tempDir
}

func MustCreateTempDir(prefix string) string {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	tempDir = dir
	RegisterCleanup(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})
	return dir
}

func RegisterCleanup(fn func()) {
	oldFn := cleanupFn
	cleanupFn = func() {
		if oldFn != nil {
			oldFn()
		}
		fn()
	}
}

func Cleanup() {
	if cleanupFn != nil {
		cleanupFn()
	}
}

func InitTempCleanup() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		Cleanup()
		os.Exit(1)
	}()
}

func CreateWorkDir(prefix string) string {
	dir := MustCreateTempDir(prefix + "-")
	EnsureDir(dir + "/images")
	EnsureDir(dir + "/configs")
	EnsureDir(dir + "/volumes")
	EnsureDir(dir + "/services")
	return dir
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func EnsureDir(dir string) error {
	if !FileExists(dir) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func EnsureFile(path string) error {
	dir := filepath.Dir(path)
	return EnsureDir(dir)
}