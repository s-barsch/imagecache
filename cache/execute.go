package cache

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"gopkg.in/gographics/imagick.v2/imagick"
)

func CacheImages(root string, opt *Options) error {
	if opt.Writer != nil {
		Writer = opt.Writer
	}
	err := CacheOriginals(root, opt)
	if err != nil {
		return err
	}

	err = DeleteCached(root)
	if err != nil {
		return err
	}

	return DeleteEmptyFolders(root)
}

func CacheOriginals(root string, opt *Options) error {
	files, err := getOriginals(root)
	if err != nil {
		return err
	}

	abort := make(chan os.Signal, 1)
	if Writer == nil {
		// only if launched from command line
		signal.Notify(abort, os.Interrupt)
	}

	imagick.Initialize()
	defer imagick.Terminate()
	for _, f := range files {
		select {
		case <-abort:
			return fmt.Errorf("PROGRAM INTERRUPTED")
		default:
			if f.base() != "cover.jpg" && !validFilename.MatchString(f.base()) {
				f, err = renameImage(f)
				if err != nil {
					return err
				}
			}
			err := CacheImage(f, opt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteEmptyFolders(root string) error {
	// two times to also delete parent dirs that turn empty after the first run
	for i := 0; i < 2; i++ {
		l, err := getEmptyFolders(root)
		if err != nil {
			return err
		}
		for _, folder := range l {
			err = os.Remove(folder.path())
			if err != nil {
				return err
			}
			Print("deleted empty cache folder %v", folder.path())
		}
	}
	return nil
}

func DeleteCached(root string) error {
	cacheFiles, err := getCached(root)
	if err != nil {
		return err
	}
	for _, cacheFile := range cacheFiles {
		if !exists(cacheFile.originalPath()) {
			err = os.Remove(cacheFile.path())
			if err != nil {
				Print("unsuccesful in deleting %v", cacheFile.path())
				continue
			}
			Print("deleted -- source gone %v", cacheFile.path())
		}
	}
	return nil
}

func sourceIsNewer(f File, size int) bool {
	sourceModTime, err := f.modtime()
	if err != nil {
		return true
	}
	cacheModTime, err := modtime(f.cacheFile(size))
	if err != nil {
		return true
	}
	if sourceModTime.Unix() < cacheModTime.Unix() {
		return false
	}
	return true
}

func modtime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("modtime: %v", err.Error())
	}
	return fi.ModTime(), nil
}

func (f File) modtime() (time.Time, error) {
	return modtime(f.path())
}

func renameImage(f File) (File, error) {
	nn, err := readExifDate(f.path())
	if err != nil {
		return "", err
	}
	nf := File(filepath.Join(f.dir(), nn))

	Print(fmt.Sprintf("renamed %v to -> %v", f.base(), nf.base())+"%v", "")
	return nf, os.Rename(f.path(), nf.path())
}

func readExifDate(fname string) (string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Optionally register camera makenote data parsing - currently Nikon and
	// Canon are supported.
	exif.RegisterParsers(mknote.All...)

	x, err := exif.Decode(f)
	if err != nil {
		log.Printf("readExifDate error: %v\n", err)
		log.Printf("path: %v\n", fname)
		return "", err
	}

	// Two convenience functions exist for date/time taken and GPS coords:
	tm, err := x.DateTime()
	if err != nil {
		return "", err
	}
	return tm.Format("060102_150405.jpg"), nil
}
