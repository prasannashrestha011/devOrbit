package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "token")
	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateToken(path)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("token was not reused: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("token mode = %v", info.Mode().Perm())
	}
}

func TestRejectsLooseTokenPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions do not apply")
	}
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(path); err == nil {
		t.Fatal("expected loose token permissions to be rejected")
	}
}

func TestRejectsTokenSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows link permissions vary by host")
	}
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(private, "token")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(filepath.Join(private, "token")); err == nil {
		t.Fatal("accepted symlinked token")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(filepath.Join(link, "other-token")); err == nil {
		t.Fatal("accepted symlinked token directory")
	}
}

func TestRejectsSharedTokenAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory mode test")
	}
	shared, err := os.MkdirTemp("/tmp", "devorbit-shared-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(shared) })
	private := filepath.Join(shared, "private")
	if err := os.MkdirAll(private, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(filepath.Join(private, "token")); err == nil {
		t.Fatal("accepted token under writable shared ancestor")
	}
}
