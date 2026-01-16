package soldati

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSoldatiManager_Create(t *testing.T) {
	// Setup: create temp directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create soldati with explicit name
	name := "vinnie"
	soldati, err := mgr.Create(name)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify soldati properties
	if soldati.Name != name {
		t.Errorf("expected name %q, got %q", name, soldati.Name)
	}
	if soldati.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if soldati.LastActive.IsZero() {
		t.Error("LastActive should not be zero")
	}

	// Verify file was created
	filePath := filepath.Join(tmpDir, name+".toml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", filePath)
	}

	// Verify we can retrieve it
	retrieved, err := mgr.Get(name)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Name != name {
		t.Errorf("retrieved name mismatch: expected %q, got %q", name, retrieved.Name)
	}
}

func TestSoldatiManager_CreateAutoName(t *testing.T) {
	// Setup: create temp directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create soldati with empty name (auto-generate)
	soldati, err := mgr.Create("")
	if err != nil {
		t.Fatalf("Create with empty name failed: %v", err)
	}

	// Verify a name was assigned
	if soldati.Name == "" {
		t.Error("expected auto-generated name, got empty string")
	}

	// Verify it's one of our mob names
	found := slices.Contains(mobNames, soldati.Name)
	// Also check for suffixed names like "vinnie-2"
	if !found {
		// Check if it starts with a mob name (for suffixed cases)
		for _, mn := range mobNames {
			if len(soldati.Name) >= len(mn) && soldati.Name[:len(mn)] == mn {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("auto-generated name %q is not a valid mob name", soldati.Name)
	}

	// Verify file was created with the auto-generated name
	filePath := filepath.Join(tmpDir, soldati.Name+".toml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", filePath)
	}
}

func TestSoldatiManager_List(t *testing.T) {
	// Setup: create temp directory
	tmpDir := t.TempDir()

	// Create manager
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Initially empty
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Create multiple soldati
	names := []string{"vinnie", "sal", "tony"}
	for _, name := range names {
		_, err := mgr.Create(name)
		if err != nil {
			t.Fatalf("Create(%s) failed: %v", name, err)
		}
	}

	// List all
	list, err = mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != len(names) {
		t.Errorf("expected %d soldati, got %d", len(names), len(list))
	}

	// Verify all names are present
	listNames := make([]string, len(list))
	for i, s := range list {
		listNames[i] = s.Name
	}
	for _, name := range names {
		if !slices.Contains(listNames, name) {
			t.Errorf("expected %q to be in list", name)
		}
	}
}

func TestSoldatiManager_Update(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create a soldati
	soldati, err := mgr.Create("vinnie")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update stats
	soldati.Stats.TasksCompleted = 5
	soldati.Stats.TasksFailed = 1
	soldati.Stats.SuccessRate = 0.833

	err = mgr.Update(soldati)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := mgr.Get("vinnie")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Stats.TasksCompleted != 5 {
		t.Errorf("expected TasksCompleted=5, got %d", retrieved.Stats.TasksCompleted)
	}
	if retrieved.Stats.TasksFailed != 1 {
		t.Errorf("expected TasksFailed=1, got %d", retrieved.Stats.TasksFailed)
	}
}

func TestSoldatiManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create a soldati
	_, err = mgr.Create("vinnie")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it exists
	_, err = mgr.Get("vinnie")
	if err != nil {
		t.Fatalf("Get failed before delete: %v", err)
	}

	// Delete it
	err = mgr.Delete("vinnie")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = mgr.Get("vinnie")
	if err == nil {
		t.Error("expected error getting deleted soldati, got nil")
	}

	// Verify file is gone
	filePath := filepath.Join(tmpDir, "vinnie.toml")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted", filePath)
	}
}

func TestSoldatiManager_GetNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to get non-existent soldati
	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent soldati, got nil")
	}
}

func TestGenerateName(t *testing.T) {
	// GenerateName should return a name from mobNames
	name := GenerateName()
	if !slices.Contains(mobNames, name) {
		t.Errorf("GenerateName returned %q which is not in mobNames", name)
	}
}

func TestGenerateUniqueName(t *testing.T) {
	// With no used names, should return a mob name
	name := GenerateUniqueName(nil)
	if !slices.Contains(mobNames, name) {
		t.Errorf("GenerateUniqueName returned %q which is not in mobNames", name)
	}

	// With some used names, should return a different one
	used := []string{"vinnie", "sal", "tony"}
	name = GenerateUniqueName(used)
	if slices.Contains(used, name) {
		t.Errorf("GenerateUniqueName returned %q which is already used", name)
	}

	// With all names used, should return a suffixed name
	allUsed := make([]string, len(mobNames))
	copy(allUsed, mobNames)
	name = GenerateUniqueName(allUsed)
	if slices.Contains(mobNames, name) {
		t.Errorf("expected suffixed name when all base names used, got %q", name)
	}
	// Should be like "vinnie-2" or similar
	if len(name) < 3 {
		t.Errorf("expected suffixed name, got %q", name)
	}
}

func TestNewManager_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "soldati", "data")

	// Directory shouldn't exist yet
	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Fatalf("expected directory to not exist initially")
	}

	// Create manager should create the directory
	_, err := NewManager(subDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}
