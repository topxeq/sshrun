package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDecodeHexIfNeeded(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "hello", want: "hello"},
		{name: "hex", input: "HEX_68656c6c6f", want: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeHexIfNeeded(tt.input)
			if err != nil {
				t.Fatalf("decodeHexIfNeeded returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("decodeHexIfNeeded(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReadCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "commands.txt")
	if err := os.WriteFile(path, []byte("date\n\nls -la\n"), 0644); err != nil {
		t.Fatalf("write command file: %v", err)
	}

	got, err := readCommands("", path)
	if err != nil {
		t.Fatalf("readCommands returned error: %v", err)
	}
	want := []string{"date", "ls -la"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readCommands = %#v, want %#v", got, want)
	}
}

func TestResolveRemoteUploadPath(t *testing.T) {
	if got := resolveRemoteUploadPath(`C:\tmp\app.bin`, "/opt/bin/", ""); got != "/opt/bin/app.bin" {
		t.Fatalf("directory upload path = %q", got)
	}
	if got := resolveRemoteUploadPath(`C:\tmp\app.bin`, "/opt/bin/custom.bin", "ignored.bin"); got != "/opt/bin/custom.bin" {
		t.Fatalf("file upload path = %q", got)
	}
}

func TestResolveLocalDownloadPath(t *testing.T) {
	dir := t.TempDir()
	got := resolveLocalDownloadPath(dir, "/remote/archive.tar.gz", "")
	want := filepath.Join(dir, "archive.tar.gz")
	if got != want {
		t.Fatalf("resolveLocalDownloadPath dir = %q, want %q", got, want)
	}

	filePath := filepath.Join(dir, "output.log")
	got = resolveLocalDownloadPath(filePath, "/remote/archive.tar.gz", "")
	if got != filePath {
		t.Fatalf("resolveLocalDownloadPath file = %q, want %q", got, filePath)
	}
}

func TestResolveStepTimeout(t *testing.T) {
	got, err := resolveStepTimeout("15s", 5*time.Second)
	if err != nil {
		t.Fatalf("resolveStepTimeout returned error: %v", err)
	}
	if got != 15*time.Second {
		t.Fatalf("resolveStepTimeout = %v, want %v", got, 15*time.Second)
	}

	got, err = resolveStepTimeout("", 5*time.Second)
	if err != nil {
		t.Fatalf("resolveStepTimeout returned error: %v", err)
	}
	if got != 5*time.Second {
		t.Fatalf("resolveStepTimeout default = %v, want %v", got, 5*time.Second)
	}
}

func TestResolveBidirectionalAction(t *testing.T) {
	local := fileState{Size: 10, ModTime: time.Unix(20, 0)}
	remote := fileState{Size: 10, ModTime: time.Unix(10, 0)}

	action, err := resolveBidirectionalAction(local, remote, "newer_wins")
	if err != nil {
		t.Fatalf("resolveBidirectionalAction returned error: %v", err)
	}
	if action != "upload" {
		t.Fatalf("action = %q, want upload", action)
	}

	action, err = resolveBidirectionalAction(local, remote, "remote_wins")
	if err != nil {
		t.Fatalf("resolveBidirectionalAction returned error: %v", err)
	}
	if action != "download" {
		t.Fatalf("action = %q, want download", action)
	}

	_, err = resolveBidirectionalAction(local, remote, "fail_on_conflict")
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestDetectBidirectionalKind(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}

	kind, err := detectBidirectionalKind(fileInfo, true, nil, false)
	if err != nil || kind != "file" {
		t.Fatalf("detect file kind = %q, err=%v", kind, err)
	}

	kind, err = detectBidirectionalKind(nil, false, dirInfo, true)
	if err != nil || kind != "dir" {
		t.Fatalf("detect dir kind = %q, err=%v", kind, err)
	}

	_, err = detectBidirectionalKind(fileInfo, true, dirInfo, true)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestResolveRemoteSyncPath(t *testing.T) {
	got := resolveRemoteSyncPath(`C:\tmp\bundle.js`, "/opt/app/", nil, false)
	if got != "/opt/app/bundle.js" {
		t.Fatalf("resolveRemoteSyncPath = %q", got)
	}
}

func TestResolveLocalSyncPath(t *testing.T) {
	dir := t.TempDir()
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}

	got := resolveLocalSyncPath(dir, "/opt/app/bundle.js", dirInfo, true)
	want := filepath.Join(dir, "bundle.js")
	if got != want {
		t.Fatalf("resolveLocalSyncPath = %q, want %q", got, want)
	}
}

func TestDecodeStringList(t *testing.T) {
	got, err := decodeStringList([]string{"dist/**,configs/*.json", "HEX_6c6f67732f2a2e6c6f67"})
	if err != nil {
		t.Fatalf("decodeStringList returned error: %v", err)
	}
	want := []string{"dist/**", "configs/*.json", "logs/*.log"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeStringList = %#v, want %#v", got, want)
	}
}

func TestShouldSyncPath(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		isDir    bool
		include  []string
		exclude  []string
		expected bool
	}{
		{name: "include nested file", rel: "dist/app.js", include: []string{"dist/**"}, expected: true},
		{name: "exclude nested file", rel: "dist/app.js", include: []string{"dist/**"}, exclude: []string{"dist/*.js"}, expected: false},
		{name: "keep parent dir for include", rel: "dist", isDir: true, include: []string{"dist/**"}, expected: true},
		{name: "skip unrelated file", rel: "docs/readme.md", include: []string{"dist/**"}, expected: false},
		{name: "basename match", rel: "logs/app.log", include: []string{"*.log"}, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSyncPath(tt.rel, tt.isDir, tt.include, tt.exclude)
			if got != tt.expected {
				t.Fatalf("shouldSyncPath(%q) = %v, want %v", tt.rel, got, tt.expected)
			}
		})
	}
}

func TestLoadDeployPlan(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "deploy.json")
	content := `{"steps":[{"name":"mkdir logs","type":"mkdir","remote_path":"/opt/app/logs"},{"type":"cmd","cmd":"echo ok","timeout":"10s"}]}`
	if err := os.WriteFile(planPath, []byte(content), 0644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	plan, err := loadDeployPlan(planPath)
	if err != nil {
		t.Fatalf("loadDeployPlan returned error: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("plan steps = %d, want 2", len(plan.Steps))
	}
	if plan.Steps[0].Name != "mkdir logs" || plan.Steps[1].Timeout != "10s" {
		t.Fatalf("unexpected plan contents: %+v", plan.Steps)
	}
}

func TestBuildHostKeyCallbackStrictRequiresKnownHosts(t *testing.T) {
	_, err := buildHostKeyCallback(true, "")
	if err == nil {
		t.Fatal("expected error when strict host key is enabled without known_hosts")
	}
}
