package skillzip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"testing"
)

func createTestZip(files map[string]string) *zip.Reader {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	return r
}

func TestWalkDir(t *testing.T) {
	// base, e := os.Open("/Users/xiaqiubo/Downloads/weworkapi_golang-master-xml.zip")
	buf, e := os.ReadFile("/Users/xiaqiubo/Desktop/test/go/messages/.codebuddy/skills/dzf.zip")
	fmt.Println(e)

	// bytes.NewReader(buf)

	// base, e := os.Open("/Users/xiaqiubo/Desktop/test/go/messages/.codebuddy/skills/dzf.zip")
	// fmt.Println(e)
	// n, e := base.Stat()
	// fmt.Println(e)
	dzf, e := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	fmt.Println(e)
	sz := NewSkillZip()
	sz.Add("dzf", dzf)
	sz.Add("skill-b", createTestZip(map[string]string{
		"config.yaml": "key: value",
	}))

	var paths []string
	err := fs.WalkDir(sz, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Printf("%s  isDir=%v\n", path, d.IsDir())
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("WalkDir returned no paths")
	}
	fmt.Println("walked paths:", paths)

	fmt.Println(fs.ReadFile(sz, "dzf/dzf/references/14获取配置信息.md"))
}
