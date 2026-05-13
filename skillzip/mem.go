package skillzip

import (
	"io/fs"
	"os"
	"strings"
	"time"
)

type filentry struct {
	children []*filentry
	content  []byte
	stat     *filestat
}

func (s *filentry) Stat() (fs.FileInfo, error) {
	return s.stat, nil
}
func (s *filentry) Read([]byte) (int, error) {
	return 0, nil
}
func (s *filentry) Close() error {
	return nil
}

func (s *filentry) Name() string               { return s.stat.name }
func (s *filentry) IsDir() bool                { return s.stat.idir }
func (s *filentry) Type() fs.FileMode          { return s.stat.mode }
func (s *filentry) Info() (fs.FileInfo, error) { return s, nil }
func (s *filentry) Size() int64                { return s.stat.size }
func (s *filentry) Mode() fs.FileMode          { return s.stat.mode }
func (s *filentry) ModTime() time.Time         { return s.stat.mod }
func (s *filentry) Sys() any                   { return s.stat.sys }

type MemFS struct {
	root *filentry
}

func NewMem(name string) *MemFS {
	return &MemFS{
		root: &filentry{
			stat: &filestat{
				name: name,
				idir: true,
				mode: fs.ModeDir | 0555,
			},
		},
	}
}

func (s *MemFS) Open(file string) (f fs.File, e error) {
	if file == "." {
		return s.root, nil
	}
	return s.find(file), nil
}

func (s *MemFS) find(file string) (f *filentry) {
	tmp := s.root
	for _, fd := range strings.Split(file, "/") { // skills/dzf/dzf/abc/aaa.md
		if tmp.stat.name != fd {
			return
		}
		for _, dir := range tmp.children {
			if dir.stat.name == fd {
				tmp = dir
			}
		}
	}
	return tmp
}

func (s *MemFS) ReadDir(dir string) (dirs []fs.DirEntry, e error) {
	temp := s.root
	if dir != "." {
		temp = s.find(dir)
	}
	if temp == nil {
		e = os.ErrNotExist
		return
	}
	for _, v := range temp.children {
		dirs = append(dirs, v)
	}
	return
}

func (s *MemFS) WriteFile(name string, content []byte) (e error) {
	tmp := s.root
	fds := strings.Split(name, "/")
	for k, fd := range fds { // skills/dzf/dzf/abc/aaa.md
		if k == 0 && s.root.stat.name != fd {
			return os.ErrNotExist
		}
		islast := len(fds)-k == 1
		for _, dir := range tmp.children {
			if dir.stat.name == fd {
				if islast && dir.stat.idir {
					return os.ErrExist
				}
				if islast {
					dir.content = content
				}
				tmp = dir
				continue
			}
		}
		var save = &filentry{
			stat: &filestat{
				name: fd,
				mode: fs.ModePerm,
				mod:  time.Now(),
				idir: islast,
			},
		}
		if islast {
			save.stat.mode = fs.ModeDir | 0o555
			save.content = content
		}
		tmp.children = append(tmp.children, save)
		tmp = save
	}
	return
}

func (s *MemFS) Remove(name string) (e error) {
	tmp := s.root
	fds := strings.Split(name, "/")
	for k, fd := range fds { // skills/dzf/dzf/abc/aaa.md
		if k == 0 && s.root.stat.name != fd {
			return os.ErrNotExist
		}
		islast := len(fds)-k == 1
		for k, dir := range tmp.children {
			if dir.stat.name == fd {
				if islast {
					old := tmp.children[k:]
					tmp.children = append([]*filentry{}, tmp.children[:k]...)
					tmp.children = append(tmp.children, old...)
				}
				tmp = dir
				continue
			}
		}
	}
	return
}

type filestat struct {
	name string
	size int64
	mode fs.FileMode
	mod  time.Time
	idir bool
	sys  any
}

func (s *filestat) Name() string       { return s.name }
func (s *filestat) Size() int64        { return s.size }
func (s *filestat) Mode() fs.FileMode  { return s.mode }
func (s *filestat) ModTime() time.Time { return s.mod }
func (s *filestat) IsDir() bool        { return s.idir }
func (s *filestat) Sys() any           { return s.sys }
