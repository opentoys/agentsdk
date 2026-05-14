package vfs

import (
	"fmt"
	"io/fs"
	"testing"
)

func TestMemFS(t *testing.T) {
	mem := NewMem("skills")

	// write files
	mem.WriteFile("hello.txt", []byte("hello world"))
	mem.WriteFile("hello/world.txt", []byte("hello nested"))
	mem.WriteFile("a/b/c.txt", []byte("deep nested"))

	// overwrite
	mem.WriteFile("hello.txt", []byte("updated"))

	// walk
	fmt.Println("=== WalkDir ===")
	err := fs.WalkDir(mem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Printf("%s  isDir=%v\n", path, d.IsDir())
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	// read file content via Open
	fmt.Println("=== Read file ===")
	f, err := mem.Open("hello.txt")
	if err != nil {
		t.Fatalf("Open hello.txt: %v", err)
	}
	buf := make([]byte, 100)
	n, _ := f.Read(buf)
	if got := string(buf[:n]); got != "updated" {
		t.Fatalf("Read hello.txt: got %q, want %q", got, "updated")
	}
	f.Close()
	fmt.Printf("hello.txt content: %s\n", string(buf[:n]))

	// read nested
	f, err = mem.Open("hello/world.txt")
	if err != nil {
		t.Fatalf("Open hello/world.txt: %v", err)
	}
	n, _ = f.Read(buf)
	if got := string(buf[:n]); got != "hello nested" {
		t.Fatalf("Read hello/world.txt: got %q, want %q", got, "hello nested")
	}
	f.Close()

	// read deep
	f, err = mem.Open("a/b/c.txt")
	if err != nil {
		t.Fatalf("Open a/b/c.txt: %v", err)
	}
	n, _ = f.Read(buf)
	if got := string(buf[:n]); got != "deep nested" {
		t.Fatalf("Read a/b/c.txt: got %q, want %q", got, "deep nested")
	}
	f.Close()

	// open nonexistent
	_, err = mem.Open("nope.txt")
	if err == nil {
		t.Fatal("Open nope.txt: expected error, got nil")
	}
	fmt.Printf("Open nope.txt: %v (expected)\n", err)

	// remove file
	err = mem.Remove("hello/world.txt")
	if err != nil {
		t.Fatalf("Remove hello/world.txt: %v", err)
	}
	_, err = mem.Open("hello/world.txt")
	if err == nil {
		t.Fatal("Remove didn't work: file still exists")
	}
	fmt.Println("Remove hello/world.txt: ok")

	// remove nonexistent
	err = mem.Remove("nope.txt")
	if err == nil {
		t.Fatal("Remove nope.txt: expected error, got nil")
	}
	fmt.Printf("Remove nope.txt: %v (expected)\n", err)

	// remove directory
	mem.WriteFile("dir/file.txt", []byte("in dir"))
	err = mem.Remove("dir/file.txt")
	if err != nil {
		t.Fatalf("Remove dir/file.txt: %v", err)
	}
	err = mem.Remove("dir")
	if err != nil {
		t.Fatalf("Remove dir: %v", err)
	}
	fmt.Println("Remove dir: ok")

	// walk after removals
	fmt.Println("=== WalkDir after removals ===")
	err = fs.WalkDir(mem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Printf("%s  isDir=%v\n", path, d.IsDir())
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir after removals failed: %v", err)
	}

	fmt.Println(fs.ReadFile(mem, "a/b/c.txt"))
}
