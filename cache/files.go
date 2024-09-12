package cache

import (
	//"io/ioutil"

	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var cacheDir = regexp.MustCompile(`.*\/img\/([0-9]{3,}|dims)`)

func isDimsFile(path string) bool {
	if l := len(path); l > 8 && path[l-8:] == ".jpg.txt" {
		return true
	}
	return false
}

func isCachedFile(path string) bool {
	if !cacheDir.MatchString(filepath.Dir(path)) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".webp":
		return true
	case ".txt":
		return isDimsFile(path)
	}
	return false
}

func getCached(root string) ([]File, error) {
	fs := []File{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		f := File(p)
		if isCachedFile(p) {
			fs = append(fs, f)
		}
		return nil
	}
	err := filepath.Walk(root, wf)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func isCacheFolderName(name string) bool {
	if name == "img" || name == "dims" {
		return true
	}
	for _, size := range sizes {
		if name == strconv.Itoa(size) {
			return true
		}
	}
	return false
}

func getEmptyFolders(root string) ([]File, error) {
	fs := []File{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		f := File(p)
		if !fi.IsDir() {
			return nil
		}
		if !isCacheFolderName(fi.Name()) {
			return nil
		}
		l, err := os.ReadDir(p)
		if err != nil {
			return nil
		}
		if len(l) > 0 {
			return nil
		}
		fs = append(fs, f)
		return nil
	}
	err := filepath.Walk(root, wf)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func getOriginals(root string) ([]File, error) {
	fs := []File{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch fi.Name() {
		case "img", "bot", ".bot", "prv":
			return filepath.SkipDir
		}
		f := File(p)
		if strings.ToLower(f.ext()) == ".jpg" { // what with png?
			fs = append(fs, f)
		}
		return nil
	}
	err := filepath.Walk(root, wf)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

type File string

func (f File) path() string {
	return string(f)
}

func (f File) base() string {
	return filepath.Base(f.path())
}

func (f File) ext() string {
	return filepath.Ext(f.path())
}

func (f File) dir() string {
	return filepath.Dir(f.path())
}

var validFolder = regexp.MustCompile("^[0-9]{2}-[0-9]{2}$")

func isMonth(path, month string) bool {
	return monthFolder(path) == month
}

func monthFolder(path string) string {
	if path == "." || path == "/" {
		return path
	}
	dir := filepath.Dir(path)
	name := filepath.Base(dir)
	if !validFolder.MatchString(name) {
		return monthFolder(dir)
	}
	return name
}

func (f File) originalPath() string {
	path := filepath.Join(
		filepath.Dir(filepath.Dir(filepath.Dir(f.path()))),
		f.base(),
	)
	switch f.ext() {
	case ".jpg":
		return strings.Replace(path, "_blur", "", -1)
	case ".webp":
		return strings.Replace(path, ".webp", ".jpg", 1)
	case ".txt":
		return strings.Replace(path, ".txt", "", 1)
	}
	panic("originalPath: invalid file extension")
}

type ByName []File

func (f ByName) Len() int           { return len(f) }
func (f ByName) Less(i, j int) bool { return f[i].path() < f[j].path() }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
