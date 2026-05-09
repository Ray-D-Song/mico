package core

import "time"

type Service struct {
	Name          string   `json:"name"`
	ContainerName string   `json:"container_name"`
	Image         string   `json:"image"`
	DependsOn     []string `json:"depends_on"`
	StartOrder    int      `json:"start_order"`
	Ports         []string `json:"ports"`
}

type PackageManifest struct {
	Version    string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
	Networks  []string  `json:"networks"`
	Services  []Service `json:"services"`
	Incremental bool      `json:"incremental"`
	BasePack     string    `json:"base_pack"`
}

type LastManifest struct {
	PackageHash string          `json:"package_hash"`
	Manifest   PackageManifest `json:"manifest"`
}

type MountInfo struct {
	Type        string `json:"type"`
	Source     string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly   bool   `json:"read_only"`
}

type ContainerMounts struct {
	ContainerName string     `json:"container_name"`
	Mounts       []MountInfo `json:"mounts"`
}
