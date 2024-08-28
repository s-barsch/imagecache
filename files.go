package main

import (
	//"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

/*
func readdir(root, path string) ([]file, error) {
	l, err := ioutil.ReadDir(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	files := []file{}
	for _, fi := range l {
		switch fi.Name() {
		case "cache", "bot", "prv", "cache.log":
			continue
		}
		files = append(files, &file(filepath.Join(path, fi.Name())))
	}
	return files, nil
}
*/

var cacheDir = regexp.MustCompile(`.*\\/cache\\/[0-9]{3,}`)

func getCached() ([]file, error) {
	fs := []file{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		f := file(p)
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

func getOriginals() ([]file, error) {
	fs := []file{}
	wf := func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch fi.Name() {
		case "cache", "bot", ".bot", "prv":
			return filepath.SkipDir
		}
		f := file(p)
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

type file string

func (f file) path() string {
	return string(f)
}

func (f file) pathWebP() string {
	return strings.Replace(f.path(), ".jpg", ".webp", 1)
}

func (f file) rel() string {
	return rel(f.path())
}

func (f file) base() string {
	return filepath.Base(f.path())
}

func (f file) ext() string {
	return filepath.Ext(f.path())
}

func (f file) dir() string {
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

/*
func (f file) folder() string {
	return filepath.Base(f.dir())
}
*/

func (f file) originalPath() string {
	path := filepath.Join(
		filepath.Dir(filepath.Dir(filepath.Dir(f.path()))),
		f.base(),
	)
	return strings.Replace(path, "_blur", "", -1)
}

type ByName []file

func (f ByName) Len() int           { return len(f) }
func (f ByName) Less(i, j int) bool { return f[i].path() < f[j].path() }
func (f ByName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
