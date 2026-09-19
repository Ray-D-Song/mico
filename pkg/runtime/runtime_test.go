package runtime

import "testing"

func TestDetectRuntimeHonorsExplicitDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")

	got := detectRuntime()
	if got.Type != Docker || got.Binary != "docker" {
		t.Fatalf("detectRuntime() = %#v, want Docker", got)
	}
}

func TestDetectRuntimeRecognizesExplicitPodmanHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///run/user/1000/podman/podman.sock")

	got := detectRuntime()
	if got.Type != Podman || got.Binary != "podman" {
		t.Fatalf("detectRuntime() = %#v, want Podman", got)
	}
}
