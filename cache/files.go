package cache

import (
	//"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var cacheDir = regexp.MustCompile(`.*\/cache\/[0-9]{3,}`)

func getCached(root string) ([]File, error) {
	fs := []File{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		f := File(p)
		if strings.ToLower(f.ext()) != ".jpg" {
			return nil
		}
		if cacheDir.MatchString(f.dir()) {
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

func getOriginals(root string) ([]File, error) {
	fs := []File{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch fi.Name() {
		case "cache", "bot", ".bot", "prv":
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

func (f File) pathWebP() string {
	return strings.Replace(f.path(), ".jpg", ".webp", 1)
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
	return strings.Replace(path, "_blur", "", -1)
}

type ByName []File

func (f ByName) Len() int           { return len(f) }
func (f ByName) Less(i, j int) bool { return f[i].path() < f[j].path() }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
