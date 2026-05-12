package utils

import (
	"os"
	"path/filepath"
)

const (
	DirMico   = ".mico"
	DirImage  = "image"
	DirConfig = "config"
	DirVolume = "volume"
	DirBind   = "bind"
)

const (
	FileLastManifest = "last_manifest.json"
	FileS3Ini        = "s3.ini"
	FileManifest     = "manifest.json"
	FileImageTar     = "image.tar"
	FileConfigJSON   = "config.json"
	FileHostJSON     = "host.json"
	FileMountsJSON   = "mounts.json"
	FileNetworkJSON  = "network.json"
	FileLog          = "pack.log"
)

func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, DirMico)
}

func GetLastManifestPath() string {
	dir := GetConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, FileLastManifest)
}

func GetS3ConfigPath() string {
	dir := GetConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, FileS3Ini)
}

func GetLogPath() string {
	dir := GetConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, FileLog)
}

func ManifestPath(workDir string) string {
	return filepath.Join(workDir, FileManifest)
}

func ServiceDir(workDir, name string) string {
	return filepath.Join(workDir, name)
}

func ServiceImageDir(workDir, name string) string {
	return filepath.Join(workDir, name, DirImage)
}

func ServiceImageTar(workDir, name string) string {
	return filepath.Join(workDir, name, DirImage, FileImageTar)
}

func ServiceConfigDir(workDir, name string) string {
	return filepath.Join(workDir, name, DirConfig)
}

func ServiceConfigJSON(workDir, name string) string {
	return filepath.Join(workDir, name, DirConfig, FileConfigJSON)
}

func ServiceHostJSON(workDir, name string) string {
	return filepath.Join(workDir, name, DirConfig, FileHostJSON)
}

func ServiceMountsJSON(workDir, name string) string {
	return filepath.Join(workDir, name, DirConfig, FileMountsJSON)
}

func ServiceNetworkJSON(workDir, name string) string {
	return filepath.Join(workDir, name, DirConfig, FileNetworkJSON)
}

func ServiceVolumeDir(workDir, name string) string {
	return filepath.Join(workDir, name, DirVolume)
}

func ServiceVolumeTar(workDir, name, volName string) string {
	return filepath.Join(workDir, name, DirVolume, volName+".tar")
}

func ServiceBindDir(workDir, name string) string {
	return filepath.Join(workDir, name, DirBind)
}

func ServiceBindTar(workDir, name, dest string) string {
	return filepath.Join(workDir, name, DirBind, EscapePath(dest)+".tar")
}

func EscapePath(path string) string {
	result := make([]byte, 0, len(path))
	for _, c := range []byte(path) {
		if c == '/' || c == ':' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
