package vfs

import (
	"bytes"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"time"
)

type filentry struct {
	children map[string]*filentry
	content  []byte
	stat     *filestat
	reader   *bytes.Reader
}

func (s *filentry) Stat() (fs.FileInfo, error) { return s.stat, nil }

func (s *filentry) Read(buf []byte) (int, error) {
	if s.stat.isdir {
		return 0, errors.New("is a directory")
	}
	if s.reader == nil {
		s.reader = bytes.NewReader(s.content)
	}
	return s.reader.Read(buf)
}

func (s *filentry) Close() error {
	s.reader = nil
	return nil
}

func (s *filentry) Name() string               { return s.stat.name }
func (s *filentry) IsDir() bool                { return s.stat.isdir }
func (s *filentry) Type() fs.FileMode          { return s.stat.mode.Type() }
func (s *filentry) Info() (fs.FileInfo, error) { return s.stat, nil }
func (s *filentry) Size() int64                { return s.stat.size }
func (s *filentry) Mode() fs.FileMode          { return s.stat.mode }
func (s *filentry) ModTime() time.Time         { return s.stat.mod }
func (s *filentry) Sys() any                   { return s.stat.sys }

type MemFS struct {
	root *filentry
}

func NewMem(names ...string) *MemFS {
	var name = "vfs"
	if len(names) > 0 {
		name = names[0]
	}
	return &MemFS{
		root: &filentry{
			children: make(map[string]*filentry),
			stat: &filestat{
				name:  name,
				isdir: true,
				mode:  fs.ModeDir | 0555,
			},
		},
	}
}

func (s *MemFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return s.root, nil
	}
	entry := s.find(name)
	if entry == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return entry, nil
}

func (s *MemFS) find(name string) *filentry {
	tmp := s.root
	for _, part := range strings.Split(name, "/") {
		child, ok := tmp.children[part]
		if !ok {
			return nil
		}
		tmp = child
	}
	return tmp
}

func (s *MemFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	entry := s.root
	if dir != "." {
		entry = s.find(dir)
	}
	if entry == nil {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrNotExist}
	}
	if !entry.stat.isdir {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: errors.New("not a directory")}
	}
	entries := make([]fs.DirEntry, 0, len(entry.children))
	for _, v := range entry.children {
		entries = append(entries, v)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

func (s *MemFS) WriteFile(name string, content []byte) error {
	if name == "" || name == "." {
		return errors.New("invalid name")
	}
	parts := strings.Split(name, "/")
	tmp := s.root
	for i, part := range parts {
		isLast := i == len(parts)-1
		child, ok := tmp.children[part]
		if ok {
			if isLast {
				if child.stat.isdir {
					return fs.ErrExist
				}
				child.content = content
				child.stat.size = int64(len(content))
				child.stat.mod = time.Now()
				return nil
			}
			tmp = child
			continue
		}
		if isLast {
			tmp.children[part] = &filentry{
				stat: &filestat{
					name:  part,
					mode:  0444,
					size:  int64(len(content)),
					mod:   time.Now(),
					isdir: false,
				},
				content: content,
			}
			return nil
		}
		dir := &filentry{
			stat: &filestat{
				name:  part,
				mode:  fs.ModeDir | 0555,
				mod:   time.Now(),
				isdir: true,
			},
			children: make(map[string]*filentry),
		}
		tmp.children[part] = dir
		tmp = dir
	}
	return nil
}

func (s *MemFS) Remove(name string) error {
	if name == "" || name == "." {
		return errors.New("invalid name")
	}
	parts := strings.Split(name, "/")
	tmp := s.root
	for i, part := range parts {
		isLast := i == len(parts)-1
		child, ok := tmp.children[part]
		if !ok {
			return fs.ErrNotExist
		}
		if isLast {
			delete(tmp.children, part)
			return nil
		}
		tmp = child
	}
	return fs.ErrNotExist
}

type filestat struct {
	name  string
	size  int64
	mode  fs.FileMode
	mod   time.Time
	isdir bool
	sys   any
}

func (s *filestat) Name() string       { return s.name }
func (s *filestat) Size() int64        { return s.size }
func (s *filestat) Mode() fs.FileMode  { return s.mode }
func (s *filestat) ModTime() time.Time { return s.mod }
func (s *filestat) IsDir() bool        { return s.isdir }
func (s *filestat) Sys() any           { return s.sys }
