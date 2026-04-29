package devcontainer

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveContainerEnvPlaceholders_ThreeLayers(t *testing.T) {
	imageEnv := map[string]string{
		"PATH": "/usr/bin:/bin",
		"HOME": "/root",
	}
	spec := &DevcontainerSpec{
		ContainerEnv: map[string]string{
			// $VAR form
			"PATH": "/opt/shims:$PATH",
			// ${VAR} form
			"MYPATH": "${PATH}:/extra",
			// ${containerEnv:VAR} form (legacy)
			"MYPATH2": "${containerEnv:PATH}:/legacy",
			// undefined var → empty
			"UNDEF": "$UNDEFINED_VAR_XYZ",
		},
		RemoteEnv: map[string]string{
			// L3 should see resolved ContainerEnv (not image PATH)
			"REMOTE_PATH": "$PATH",
		},
	}
	spec.ResolveContainerEnvPlaceholders(imageEnv)

	cases := []struct {
		key  string
		env  map[string]string
		want string
	}{
		{"PATH", spec.ContainerEnv, "/opt/shims:/usr/bin:/bin"},
		{"MYPATH", spec.ContainerEnv, "/usr/bin:/bin:/extra"},
		{"MYPATH2", spec.ContainerEnv, "/usr/bin:/bin:/legacy"},
		{"UNDEF", spec.ContainerEnv, ""},
		// RemoteEnv PATH resolves against L1∪resolved-L2 (containerEnv PATH wins)
		{"REMOTE_PATH", spec.RemoteEnv, "/opt/shims:/usr/bin:/bin"},
	}
	for _, c := range cases {
		if got := c.env[c.key]; got != c.want {
			t.Errorf("%s = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestLoadSpec_ImageField(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"myproject:dev"}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_BuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile","name":"myproject:dev"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "myproject:dev" {
		t.Errorf("Image = %q, want myproject:dev", spec.Image)
	}
}

func TestLoadSpec_ImagePrecedenceOverBuildName(t *testing.T) {
	dir := setupProjectDC(t, `{"image":"top:v1","build":{"name":"build:v2"}}`)
	spec, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Image != "top:v1" {
		t.Errorf("Image = %q, want top:v1 (image: takes precedence)", spec.Image)
	}
}

func TestLoadSpec_MissingImage_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"containerEnv":{"FOO":"bar"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}

func TestLoadSpec_BuildWithoutName_Error(t *testing.T) {
	dir := setupProjectDC(t, `{"build":{"dockerfile":"Dockerfile"}}`)
	_, err := LoadSpec(dir, filepath.Join(dir, devcontainerSubdir))
	if !errors.Is(err, ErrMissingImage) {
		t.Errorf("expected ErrMissingImage, got %v", err)
	}
}
