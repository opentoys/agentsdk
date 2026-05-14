package vfs

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
)

func CreateZip(buf io.Writer, files map[string][]byte) (e error) {
	w := zip.NewWriter(buf)
	for name, content := range files {
		f, e := w.Create(name)
		if e != nil {
			return e
		}
		_, e = f.Write(content)
		if e != nil {
			return e
		}
	}
	return w.Close()
}

func ParseZip(buf []byte) (files map[string][]byte, e error) {
	r, e := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if e != nil {
		return
	}
	files = make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = content
	}
	return
}

func (s *MemFS) Export(buf io.Writer) (e error) {
	var data = map[string][]byte{}
	e = fs.WalkDir(s, ".", func(path string, d fs.DirEntry, err error) (e error) {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return
		}
		data[path], e = fs.ReadFile(s, path)
		return
	})
	if e != nil {
		return
	}
	return CreateZip(buf, data)
}

func (s *MemFS) Merge(prefix string, sub fs.FS) (e error) {
	e = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) (e error) {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return
		}
		buf, e := fs.ReadFile(s, path)
		if e != nil {
			return
		}
		s.WriteFile(prefix+path, buf)
		return
	})
	return
}
