package configs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const manifestPath = "aibbe.nm-host.json"

type NMHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func TestNMHostManifest_IsStrictlyValidJSON(t *testing.T) {
	data := loadManifestBytes(t)

	var manifest NMHostManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("decode manifest with strict schema: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest to map: %v", err)
	}

	if got, want := len(raw), 5; got != want {
		t.Fatalf("manifest field count = %d, want %d", got, want)
	}
}

func TestNMHostManifest_SchemaMatchesSpec(t *testing.T) {
	manifest := loadManifest(t)

	if got, want := manifest.Name, "aibbe"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}

	if manifest.Description == "" {
		t.Fatal("description must not be empty")
	}

	if got, want := manifest.Path, "/home/gary/dev/ai-browser-bridge-extension/daemon/aibbe"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}

	if got, want := len(manifest.AllowedOrigins), 1; got != want {
		t.Fatalf("allowed_origins length = %d, want %d", got, want)
	}

	if got, want := manifest.AllowedOrigins[0], "chrome-extension://bedlojjaiogmaefoadfpdecgajipcpgj/"; got != want {
		t.Fatalf("allowed_origins[0] = %q, want %q", got, want)
	}
}

func TestNMHostManifest_PathIsAbsolute(t *testing.T) {
	manifest := loadManifest(t)

	if !filepath.IsAbs(manifest.Path) {
		t.Fatalf("path must be absolute, got %q", manifest.Path)
	}
}

func TestNMHostManifest_TypeIsStdio(t *testing.T) {
	manifest := loadManifest(t)

	if got, want := manifest.Type, "stdio"; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
}

func TestNMHostManifest_AllowedOriginHasTrailingSlash(t *testing.T) {
	manifest := loadManifest(t)

	for _, origin := range manifest.AllowedOrigins {
		if !strings.HasSuffix(origin, "/") {
			t.Fatalf("allowed origin must end with trailing slash: %q", origin)
		}
	}
}

func TestNMHostManifest_BinaryExistsAndIsExecutable(t *testing.T) {
	manifest := loadManifest(t)

	info, err := os.Stat(manifest.Path)
	if err != nil {
		t.Fatalf("stat manifest binary path: %v", err)
	}

	if info.IsDir() {
		t.Fatalf("manifest path points to a directory, want executable file: %q", manifest.Path)
	}

	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("manifest binary is not executable: mode=%#o path=%q", info.Mode().Perm(), manifest.Path)
	}
}

func loadManifest(t *testing.T) NMHostManifest {
	t.Helper()

	data := loadManifestBytes(t)

	var manifest NMHostManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	return manifest
}

func loadManifestBytes(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(".", manifestPath))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	return data
}
