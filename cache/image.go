package cache

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/gographics/imagick.v3/imagick"
)

const CacheSectionFolder = "cache"

func dirRegex(dir string) string {
	return fmt.Sprintf("/%v/", dir)
}

type Options struct {
	RerunFolder string
	RerunSize   int
	RerunDims   bool
	Writer      io.Writer
}

var Writer io.Writer

var validFilename = regexp.MustCompile("^[0-9]{6}_[0-9]{6}[a-z\u00E0-\u00FC-+]*\\.[a-z]+$")
var numFilename = regexp.MustCompile("/img/[0-9]{3,4}/")

var sizes = []int{320, 480, 640, 800, 960, 1280, 1440, 1600, 1920, 2560, 3200}

var sharpen = map[int]float64{
	320:  0.5,
	480:  0.5,
	640:  0.6,
	800:  0.8,
	960:  0.8,
	1280: 0.8,
	1600: 0.8,
	1920: 0.8,
	2560: 0.8,
	3200: 0.8,
}

func Print(msg, path string) {
	var (
		lb string
		mw io.Writer
	)
	size := numFilename.FindString(path)
	if Writer == nil {
		mw = os.Stdout
		lb = "\n"
		path = Cap(path)
	} else {
		mw = Writer
		path = filepath.Base(path)
	}
	if size != "" {
		size = strings.Trim(size, "/img")
		fmt.Fprintf(mw, msg+"\t(%v)"+lb, path, size)
		return
	}
	fmt.Fprintf(mw, msg+lb, path)
}

type locker struct {
	mu sync.Mutex
}

func (l *locker) createFolder(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if exists(path) {
		return nil
	}

	err := os.Mkdir(path, 0755)
	if err != nil {
		return err
	}

	Print("created %v (folder)", path)
	return nil
}

func CacheImage(f File, locker *locker, opt *Options) error {
	if opt.Writer != nil {
		Writer = opt.Writer
	}
	err := locker.createFolder(f.cacheFolder())
	if err != nil {
		return err
	}

	if !exists(f.dimsFile()) || opt.RerunDims || sourceIsNewer(f, 1600) {
		err := f.createDimsFile(locker)
		if err != nil {
			return err
		}
		Print("created dims file %v", f.dimsFile())
	}

	sizeMap := make(map[int]struct{})
	for _, size := range sizes {
		// randomize execution order
		sizeMap[size] = struct{}{}
	}

	for size := range sizeMap {
		err := locker.createFolder(f.sizeFolder(size))
		if err != nil {
			return err
		}
		rerunCache := false
		if x := strings.Index(f.path(), dirRegex(CacheSectionFolder)); opt.RerunFolder == CacheSectionFolder && x != -1 {
			rerunCache = true
		}
		if !exists(f.cacheFilePath(size, JPEG)) || size == opt.RerunSize || rerunCache ||
			isMonth(f.path(), opt.RerunFolder) || sourceIsNewer(f, size) {
			err := f.createCacheFile(size)
			if err != nil {
				return err
			}
			continue
		}
		Print("skipping: %v -- already cached", f.cacheFilePath(size, JPEG))
	}

	return nil
}

func (f File) createCacheFile(size int) error {
	if ext := f.ext(); ext != JPEG.Ext() && ext != AVIF.Ext() && ext != PNG.Ext() {
		return fmt.Errorf("source file should be jpeg, png, or avif")
	}

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err := mw.ReadImage(f.path())
	if err != nil {
		panic(err)
	}

	err = mw.AutoOrientImage()
	if err != nil {
		return err
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	orientation := ""

	if w >= h {
		orientation = "landscape"
	} else {
		orientation = "portrait"
	}

	err = mw.SetColorspace(imagick.COLORSPACE_RGB)
	if err != nil {
		return err
	}
	err = mw.SetInterlaceScheme(imagick.INTERLACE_JPEG)
	if err != nil {
		return err
	}
	err = mw.StripImage()
	if err != nil {
		return err
	}

	if orientation == "landscape" && w > uint(size) || orientation == "portrait" && h > uint(size) {
		var newW, newH uint
		if orientation == "portrait" {
			// scale by height
			newH = uint(size)
			newW = uint(float64(w) * float64(size) / float64(h))
		} else {
			// scale by width
			newW = uint(size)
			newH = uint(float64(h) * float64(size) / float64(w))
		}

		err := mw.ResizeImage(newW, newH, imagick.FILTER_LANCZOS)
		if err != nil {
			return err
		}

		// dont sharpen nexus images with image ratio 4:3
		if math.Trunc((float64(max(w, h))/float64(min(w, h)))*100) == 133 &&
			f.base()[:4] < "1903" {
			err = mw.SharpenImage(0, 0.5)
			log.Println("NEXUS, decreased sharpen")
			if err != nil {
				return err
			}
		} else {
			err = mw.SharpenImage(0, sharpen[size])
			if err != nil {
				return err
			}
		}
	}

	compressionSettings := map[Format]uint{
		JPEG: 90,
		AVIF: 100,
	}

	for _, format := range CacheFormats() {
		wmw := mw.Clone()
		defer wmw.Destroy()

		quality, ok := compressionSettings[format]
		if !ok {
			panic("compression settings not set")
		}

		err = mw.SetImageCompressionQuality(quality)
		if err != nil {
			return err
		}

		err = wmw.SetImageFormat(format.MagickFormat())
		if err != nil {
			return err
		}

		p := f.cacheFilePath(size, format)
		err = wmw.WriteImage(p)
		if err != nil {
			return err
		}

		Print("cached: %v", p)
	}

	return nil
}

func (f File) createDimsFile(locker *locker) error {
	err := locker.createFolder(f.dimsFolder())
	if err != nil {
		return err
	}

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err = mw.ReadImage(f.path())
	if err != nil {
		panic(err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	return os.WriteFile(f.dimsFile(), []byte(fmt.Sprintf("%dx%d", w, h)), 0644)
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint) uint {
	if a > b {
		return a
	}
	return b
}

func (f File) cacheFolder() string {
	return filepath.Join(f.dir(), "img")
}

func (f File) sizeFolder(size int) string {
	return filepath.Join(f.cacheFolder(), strconv.FormatInt(int64(size), 10))
}

/*
func (f File) cacheFileBlur(size int) string {
	path := f.cacheFile(size)
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		panic("invalid path")
	}
	return path[:i] + "_blur" + path[i:]
}
*/

func (f File) cacheFilePathKeepExt(size int) string {
	return filepath.Join(f.sizeFolder(size), f.base())
}

func (f File) cacheFilePath(size int, format Format) string {
	path := f.cacheFilePathKeepExt(size)
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		panic("invalid path")
	}
	return path[:i] + format.Ext()
}

func (f File) dimsFolder() string {
	return filepath.Join(f.cacheFolder(), "dims")
}

func (f File) dimsFile() string {
	return filepath.Join(f.dimsFolder(), f.base()+".txt")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func multiExists(paths []string) bool {
	for _, p := range paths {
		if exists(p) {
			return true
		}
	}

	return false
}

func Cap(path string) string {
	const data = "/data"
	const l = len(data)
	if i := strings.Index(path, data); i > 0+l {
		return path[i+l:]
	}
	return path
}
