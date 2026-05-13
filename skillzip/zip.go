package skillzip

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

func ReadFile(name string) (r *zip.Reader, e error) {
	buf, e := os.ReadFile(name)
	if e != nil {
		return
	}
	return zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
}

func ReadURL(url string) (r *zip.Reader, e error) {
	resp, e := http.Get(url)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	buf, e := io.ReadAll(resp.Body)
	if e != nil {
		return
	}
	return zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
}

func CreateZip(content map[string]string) *zip.Reader {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range content {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	r, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	return r
}

type SkillZip struct {
	data map[string]*zip.Reader
}

func New() *SkillZip {
	return &SkillZip{data: make(map[string]*zip.Reader)}
}

func (s *SkillZip) Add(name string, data *zip.Reader) {
	s.data[name] = data
}

func (s *SkillZip) Open(file string) (f fs.File, e error) {
	if file == "." {
		return &dirFile{skillstat: &skillstat{name: "skills", mode: fs.ModeDir | 0555, idir: true}}, nil
	}
	paths := strings.Split(file, "/")
	r := s.data[paths[0]]
	if r == nil {
		return nil, os.ErrNotExist
	}
	subPath := strings.Join(paths[1:], "/")
	if subPath == "" {
		subPath = "."
	}
	return r.Open(subPath)
}

func (s *SkillZip) ReadDir(dir string) ([]fs.DirEntry, error) {
	if dir == "." {
		entries := make([]fs.DirEntry, 0, len(s.data))
		for name := range s.data {
			entries = append(entries, &skillDirEntry{stat: &skillstat{name: name, mode: fs.ModeDir | 0555, idir: true}})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		return entries, nil
	}
	paths := strings.Split(dir, "/")
	r := s.data[paths[0]]
	if r == nil {
		return nil, os.ErrNotExist
	}
	subDir := strings.Join(paths[1:], "/")
	if subDir == "" {
		subDir = "."
	}
	return fs.ReadDir(r, subDir)
}

type skillDirEntry struct {
	stat *skillstat
}

func (d *skillDirEntry) Name() string               { return d.stat.name }
func (d *skillDirEntry) IsDir() bool                { return d.stat.idir }
func (d *skillDirEntry) Type() fs.FileMode          { return d.stat.mode.Type() }
func (d *skillDirEntry) Info() (fs.FileInfo, error) { return d.stat, nil }

type dirFile struct {
	*skillstat
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.skillstat, nil }
func (d *dirFile) Read([]byte) (int, error)   { return 0, nil }
func (d *dirFile) Close() error               { return nil }

type skillstat struct {
	name string
	size int64
	mode fs.FileMode
	mod  time.Time
	idir bool
	sys  any
}

func (s *skillstat) Name() string       { return s.name }
func (s *skillstat) Size() int64        { return s.size }
func (s *skillstat) Mode() fs.FileMode  { return s.mode }
func (s *skillstat) ModTime() time.Time { return s.mod }
func (s *skillstat) IsDir() bool        { return s.idir }
func (s *skillstat) Sys() any           { return s.sys }
