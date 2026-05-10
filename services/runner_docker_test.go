package services

import (
	"strings"
	"testing"
	"time"
)

func TestResolveDockerMemoryLimit(t *testing.T) {
	if got := resolveDockerMemoryLimit(0); got != "268435456" {
		t.Fatalf("expected default memory bytes, got %s", got)
	}

	limit := resolveDockerMemoryLimit(262144)
	if limit != "268435456" {
		t.Fatalf("expected exact bytes for 256mb, got %s", limit)
	}

	lowLimit := resolveDockerMemoryLimit(1024)
	if lowLimit != "134217728" {
		t.Fatalf("expected minimum 128mb floor, got %s", lowLimit)
	}
}

func TestResolveCompileMemoryLimit(t *testing.T) {
	runner := &DockerRunner{}

	if got := runner.resolveCompileMemoryLimit(262144); got != "536870912" {
		t.Fatalf("expected compile floor of 512mb, got %s", got)
	}

	if got := runner.resolveCompileMemoryLimit(1048576); got != "1073741824" {
		t.Fatalf("expected larger problem memory limit to win, got %s", got)
	}

	runner.CompileMemMb = 768
	if got := runner.resolveCompileMemoryLimit(262144); got != "805306368" {
		t.Fatalf("expected compile override of 768mb, got %s", got)
	}
}

func TestBuildDockerRunArgsIncludesSandboxFlags(t *testing.T) {
	runner := &DockerRunner{
		DockerBin:   "docker",
		Image:       "discover-judge:latest",
		CPULimit:    "1.5",
		TmpfsSizeMb: 128,
		PidsLimit:   64,
	}

	args := runner.buildDockerRunArgs(dockerArgsInput{
		ContainerName: "judge-1-1",
		Workspace:     "/tmp/work",
		Command:       []string{"./main"},
		Environment:   []string{"GO111MODULE=off"},
		MemoryLimit:   "268435456",
	})
	text := strings.Join(args, " ")

	assertContains(t, text, "--network none")
	assertContains(t, text, "-i")
	assertContains(t, text, "--read-only")
	assertContains(t, text, "--tmpfs /tmp:rw,nosuid,nodev,noexec,size=128m")
	assertContains(t, text, "--cap-drop ALL")
	assertContains(t, text, "--security-opt no-new-privileges")
	assertContains(t, text, "--memory 268435456")
	assertContains(t, text, "--memory-swap 268435456")
	assertContains(t, text, "--cpus 1.5")
	assertContains(t, text, "-v /tmp/work:/workspace:rw")
	assertContains(t, text, "-e HOME=/workspace")
	assertContains(t, text, "-e TMPDIR=/workspace")
	assertContains(t, text, "-e GOCACHE=/workspace/.cache/go-build")
}

func TestBuildDockerRunArgsUsesVolumeMountWhenProvided(t *testing.T) {
	runner := &DockerRunner{
		DockerBin:   "docker",
		Image:       "discover-judge:latest",
		CPULimit:    "1",
		TmpfsSizeMb: 64,
		PidsLimit:   128,
	}

	args := runner.buildDockerRunArgs(dockerArgsInput{
		ContainerName: "judge-2-1",
		VolumeName:    "judge-vol-2-1",
		Workspace:     "/tmp/unused",
		Command:       []string{"./main"},
		MemoryLimit:   "268435456",
	})
	text := strings.Join(args, " ")

	assertContains(t, text, "-v judge-vol-2-1:/workspace:rw")
	if strings.Contains(text, "/tmp/unused:/workspace:rw") {
		t.Fatalf("expected named volume mount to replace workspace bind mount, got %q", text)
	}
}

func TestResolveCompileTimeout(t *testing.T) {
	runner := &DockerRunner{}
	timeout := runner.resolveCompileTimeout(200)
	if timeout < 10*time.Second {
		t.Fatalf("expected minimum compile timeout, got %s", timeout)
	}

	timeout = runner.resolveCompileTimeout(60000)
	if timeout != 120*time.Second {
		t.Fatalf("expected max compile timeout 120s, got %s", timeout)
	}

	runner.CompileLimit = 12 * time.Second
	timeout = runner.resolveCompileTimeout(1000)
	if timeout != 12*time.Second {
		t.Fatalf("expected configured compile timeout, got %s", timeout)
	}
}

func TestResolveDockerImageForLanguage(t *testing.T) {
	runner := &DockerRunner{}

	if got := runner.resolveDockerImageForLanguage("go"); got != "golang:1.25-bookworm" {
		t.Fatalf("expected go image, got %s", got)
	}
	if got := runner.resolveDockerImageForLanguage("c"); got != "gcc:14-bookworm" {
		t.Fatalf("expected c image, got %s", got)
	}
	if got := runner.resolveDockerImageForLanguage("cpp"); got != "gcc:14-bookworm" {
		t.Fatalf("expected cpp image, got %s", got)
	}
	if got := runner.resolveDockerImageForLanguage("java"); got != "eclipse-temurin:21-jdk" {
		t.Fatalf("expected java image, got %s", got)
	}

	runner.GoImage = "custom/go:latest"
	if got := runner.resolveDockerImageForLanguage("go"); got != "custom/go:latest" {
		t.Fatalf("expected go override image, got %s", got)
	}
}

func assertContains(t *testing.T, text string, expected string) {
	t.Helper()
	if !strings.Contains(text, expected) {
		t.Fatalf("expected %q in %q", expected, text)
	}
}
