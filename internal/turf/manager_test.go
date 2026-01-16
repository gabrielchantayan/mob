package turf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTurfManager_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake project directory
	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = mgr.Add(projectDir, "my-project", "main")
	if err != nil {
		t.Fatalf("failed to add turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 1 {
		t.Errorf("expected 1 turf, got %d", len(turfs))
	}
	if turfs[0].Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", turfs[0].Name)
	}
}

func TestTurfManager_Remove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Add(projectDir, "my-project", "main")
	err = mgr.Remove("my-project")
	if err != nil {
		t.Fatalf("failed to remove turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 0 {
		t.Errorf("expected 0 turfs, got %d", len(turfs))
	}
}

func TestTurfManager_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Add(projectDir, "my-project", "main")

	// Test successful get
	turf, err := mgr.Get("my-project")
	if err != nil {
		t.Fatalf("failed to get turf: %v", err)
	}
	if turf.Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", turf.Name)
	}
	if turf.MainBranch != "main" {
		t.Errorf("expected main branch 'main', got '%s'", turf.MainBranch)
	}

	// Test that returned pointer refers to actual slice element
	turf.MainBranch = "develop"
	turf2, _ := mgr.Get("my-project")
	if turf2.MainBranch != "develop" {
		t.Errorf("expected main branch 'develop' after modification, got '%s'", turf2.MainBranch)
	}

	// Test get non-existent turf
	_, err = mgr.Get("non-existent")
	if err == nil {
		t.Error("expected error when getting non-existent turf")
	}
}

func TestTurfManager_Add_DuplicateName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir1 := filepath.Join(tmpDir, "project1")
	projectDir2 := filepath.Join(tmpDir, "project2")
	if err := os.MkdirAll(projectDir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir2, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.Add(projectDir1, "my-project", "main")
	if err != nil {
		t.Fatalf("failed to add first turf: %v", err)
	}

	// Try to add with duplicate name
	err = mgr.Add(projectDir2, "my-project", "main")
	if err == nil {
		t.Error("expected error when adding turf with duplicate name")
	}
}

func TestTurfManager_Add_NonExistentPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	// Try to add with non-existent path
	err = mgr.Add("/path/that/does/not/exist", "my-project", "main")
	if err == nil {
		t.Error("expected error when adding turf with non-existent path")
	}
}

func TestTurfManager_Remove_NonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	// Try to remove non-existent turf
	err = mgr.Remove("non-existent")
	if err == nil {
		t.Error("expected error when removing non-existent turf")
	}
}

func TestTurfManager_List_ReturnsCopy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Add(projectDir, "my-project", "main")

	// Get the list and modify it
	turfs := mgr.List()
	turfs[0].Name = "modified-name"

	// Get the list again and verify original is unchanged
	turfs2 := mgr.List()
	if turfs2[0].Name != "my-project" {
		t.Errorf("expected original name 'my-project', got '%s' - List() should return a copy", turfs2[0].Name)
	}
}
