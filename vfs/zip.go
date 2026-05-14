package vfs

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

func ZipReadFile(name string) (r *zip.Reader, e error) {
	buf, e := os.ReadFile(name)
	if e != nil {
		return
	}
	return zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
}

func ZipReadURL(url string) (r *zip.Reader, e error) {
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

func ZipCreate(content map[string]string) *zip.Reader {
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

type ZipFS struct {
	data map[string]*zip.Reader
}

func NewZip() *ZipFS {
	return &ZipFS{data: make(map[string]*zip.Reader)}
}

func (s *ZipFS) Add(name string, data *zip.Reader) {
	s.data[name] = data
}

func (s *ZipFS) Open(file string) (f fs.File, e error) {
	if file == "." {
		return &fsDirFile{fstat: &fstat{name: "skills", mode: fs.ModeDir | 0555, idir: true}}, nil
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

func (s *ZipFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	if dir == "." {
		entries := make([]fs.DirEntry, 0, len(s.data))
		for name := range s.data {
			entries = append(entries, &fsDirEntry{stat: &fstat{name: name, mode: fs.ModeDir | 0555, idir: true}})
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

type fsDirEntry struct {
	stat *fstat
}

func (d *fsDirEntry) Name() string               { return d.stat.name }
func (d *fsDirEntry) IsDir() bool                { return d.stat.idir }
func (d *fsDirEntry) Type() fs.FileMode          { return d.stat.mode.Type() }
func (d *fsDirEntry) Info() (fs.FileInfo, error) { return d.stat, nil }

type fsDirFile struct {
	*fstat
}

func (d *fsDirFile) Stat() (fs.FileInfo, error) { return d.fstat, nil }
func (d *fsDirFile) Read([]byte) (int, error)   { return 0, nil }
func (d *fsDirFile) Close() error               { return nil }

type fstat struct {
	name string
	size int64
	mode fs.FileMode
	mod  time.Time
	idir bool
	sys  any
}

func (s *fstat) Name() string       { return s.name }
func (s *fstat) Size() int64        { return s.size }
func (s *fstat) Mode() fs.FileMode  { return s.mode }
func (s *fstat) ModTime() time.Time { return s.mod }
func (s *fstat) IsDir() bool        { return s.idir }
func (s *fstat) Sys() any           { return s.sys }
