package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"golang.org/x/sync/errgroup"
	"gopkg.in/gographics/imagick.v3/imagick"
)

func CacheImages(root string, opt *Options) error {
	if opt.Writer != nil {
		Writer = opt.Writer
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := CacheOriginals(ctx, root, opt)
	if err != nil {
		return err
	}

	err = DeleteCached(root)
	if err != nil {
		return err
	}

	return DeleteEmptyFolders(root)
}

func CacheOriginals(ctx context.Context, root string, opt *Options) error {
	g, ctx := errgroup.WithContext(ctx)

	files, err := getOriginals(root)
	if err != nil {
		return err
	}

	locker := &locker{}
	buf := make(chan struct{}, 10)

	imagick.Initialize()
	defer imagick.Terminate()
	for i, f := range files {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case buf <- struct{}{}:
				fmt.Printf("queue #%v\n", i)
				defer func() {
					<-buf
				}()
				return renameAndCache(f, locker, opt)
			}
		})
	}

	return g.Wait()
}

func renameAndCache(f File, locker *locker, opt *Options) error {
	var err error
	if f.base() != "cover.jpg" && !validFilename.MatchString(f.base()) {
		f, err = renameImage(f)
		if err != nil {
			return err
		}
	}
	return CacheImage(f, locker, opt)
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
		shouldDelete := !multiExists(cacheFile.originalPaths())
		if !shouldDelete {
			name := filepath.Base(cacheFile.path())
			shouldDelete = filepath.Ext(name) == ".webp" || strings.Contains(name, "_blur")
		}
		if shouldDelete {
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
	cacheModTime, err := modtime(f.cacheFilePath(size, JPEG))
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
