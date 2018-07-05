package test

import (
	"io"
	"os"

	"github.com/spf13/afero"

	"github.com/heptio/ark/pkg/util/filesystem"
)

type FakeFileSystem struct {
	fs afero.Fs

	ReadDirCalls []string
}

func NewFakeFileSystem() *FakeFileSystem {
	return &FakeFileSystem{
		fs: afero.NewMemMapFs(),
	}
}

func (fs *FakeFileSystem) TempDir(dir, prefix string) (string, error) {
	return afero.TempDir(fs.fs, dir, prefix)
}

func (fs *FakeFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return fs.fs.MkdirAll(path, perm)
}

func (fs *FakeFileSystem) Create(name string) (io.WriteCloser, error) {
	return fs.fs.Create(name)
}

func (fs *FakeFileSystem) RemoveAll(path string) error {
	return fs.fs.RemoveAll(path)
}

func (fs *FakeFileSystem) ReadDir(dirname string) ([]os.FileInfo, error) {
	fs.ReadDirCalls = append(fs.ReadDirCalls, dirname)
	return afero.ReadDir(fs.fs, dirname)
}

func (fs *FakeFileSystem) ReadFile(filename string) ([]byte, error) {
	return afero.ReadFile(fs.fs, filename)
}

func (fs *FakeFileSystem) DirExists(path string) (bool, error) {
	return afero.DirExists(fs.fs, path)
}

func (fs *FakeFileSystem) WithFile(path string, data []byte) *FakeFileSystem {
	file, _ := fs.fs.Create(path)
	file.Write(data)
	file.Close()

	return fs
}

func (fs *FakeFileSystem) WithDirectory(path string) *FakeFileSystem {
	fs.fs.MkdirAll(path, 0755)
	return fs
}

func (fs *FakeFileSystem) WithDirectories(path ...string) *FakeFileSystem {
	for _, dir := range path {
		fs = fs.WithDirectory(dir)
	}

	return fs
}

func (fs *FakeFileSystem) TempFile(dir, prefix string) (filesystem.NameWriteCloser, error) {
	return afero.TempFile(fs.fs, dir, prefix)
}
