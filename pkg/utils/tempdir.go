package utils

import (
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

var (
	tempDir    string
	tempDirMu  sync.Mutex
	tempDirs   = make(map[string]struct{})
	cleanupFns []func()
)

func SetTempDir(dir string) {
	tempDirMu.Lock()
	defer tempDirMu.Unlock()
	tempDir = dir
}

func GetTempDir() string {
	tempDirMu.Lock()
	defer tempDirMu.Unlock()
	return tempDir
}

func MustCreateTempDir(prefix string) string {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	tempDirMu.Lock()
	tempDir = dir
	tempDirs[dir] = struct{}{}
	tempDirMu.Unlock()
	return dir
}

func RegisterCleanup(fn func()) {
	if fn == nil {
		return
	}
	tempDirMu.Lock()
	cleanupFns = append(cleanupFns, fn)
	tempDirMu.Unlock()
}

func Cleanup() {
	tempDirMu.Lock()
	dirs := make([]string, 0, len(tempDirs))
	for dir := range tempDirs {
		dirs = append(dirs, dir)
	}
	tempDirs = make(map[string]struct{})
	tempDir = ""
	fns := append([]func(){}, cleanupFns...)
	cleanupFns = nil
	tempDirMu.Unlock()

	for _, fn := range fns {
		fn()
	}
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}

func RemoveWorkDir(dir string) {
	if dir == "" {
		return
	}
	tempDirMu.Lock()
	delete(tempDirs, dir)
	if tempDir == dir {
		tempDir = ""
	}
	tempDirMu.Unlock()
	_ = os.RemoveAll(dir)
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
	return dir
}

func CreateServiceDir(workDir, serviceName string) string {
	servicePath := filepath.Join(workDir, serviceName)
	EnsureDir(ServiceImageDir(workDir, serviceName))
	EnsureDir(ServiceConfigDir(workDir, serviceName))
	EnsureDir(ServiceVolumeDir(workDir, serviceName))
	return servicePath
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
