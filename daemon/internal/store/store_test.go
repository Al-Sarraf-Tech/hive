package store

import (
	"os"
	"testing"
)

func TestStoreOpenClose(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(dir + "/hive.db"); err != nil {
		t.Fatal("database file not created")
	}
}

func TestStorePutGet(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	err = s.Put("services", "web", []byte(`{"image":"nginx"}`))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := s.Get("services", "web")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != `{"image":"nginx"}` {
		t.Errorf("unexpected value: %s", val)
	}
}

func TestStoreGetMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	val, err := s.Get("services", "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for missing key, got %s", val)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	_ = s.Put("services", "web", []byte("data"))

	err = s.Delete("services", "web")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	val, _ := s.Get("services", "web")
	if val != nil {
		t.Error("expected nil after delete")
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	_ = s.Put("services", "web", []byte("a"))
	_ = s.Put("services", "api", []byte("b"))
	_ = s.Put("services", "db", []byte("c"))

	keys, err := s.List("services")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}
