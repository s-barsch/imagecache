package main

// TODO: there is an issue if one image of an already cached series is deleted. series doesnt show up.

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"gopkg.in/gographics/imagick.v2/imagick"
)

// var root = "/srv/rg-s/st/data"
var root = "/Volumes/External/srv/rg-s/st/data/kine"

// var root = "/Volumes/External/srv/rg-s/st/data/graph"
var rootl = len(root)

//var logfile = root + "/cache.log"

type options struct {
	rerunFolder string
	rerunSize   int
	rerunDims   bool
}

var abort chan os.Signal

func main() {
	rerunFolder := flag.String("rerun", "", "which folder should be freshly cached? eg: 17-07. write \"all\" for everything.")
	rerunSize := flag.Int("size", 0, "specify if a specific size should be recached.")
	rerunDims := flag.Bool("rerunDims", false, "recreate dimension files for all images.")
	flag.Parse()

	opt := &options{
		rerunFolder: *rerunFolder,
		rerunSize:   *rerunSize,
		rerunDims:   *rerunDims,
	}

	abort = make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt)

	err := cacheImages(opt)
	fmt.Println(err)
}

var sizes = []int{320, 480, 640, 800, 960, 1280, 1600, 1920, 2560, 3200}

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

func rel(path string) string {
	if len(path) > rootl {
		return path[rootl:]
	}
	return path
}

// var validFilename = regexp.MustCompile("^[0-9]+_[0-9]+.jpg$")
var validFilename = regexp.MustCompile("^[0-9]{6}_[0-9]{6}[a-z\u00E0-\u00FC-+]*\\.[a-z]+$")

func cacheImages(opt *options) error {
	err := cacheOriginals(opt)
	if err != nil {
		return err
	}

	return deleteCached()
}

func (f file) cacheFolder() string {
	return filepath.Join(f.dir(), "cache")
}

func (f file) sizeFolder(size int) string {
	return filepath.Join(f.cacheFolder(), strconv.FormatInt(int64(size), 10))
}

func (f file) cacheFileBlur(size int) string {
	path := f.cacheFile(size)
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		panic("invalid path")
	}
	return path[:i] + "_blur" + path[i:]
}

func (f file) cacheFile(size int) string {
	return filepath.Join(f.sizeFolder(size), f.base())
}

func (f file) cacheFileWebP(size int) string {
	path := f.cacheFile(size)
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		panic("invalid path")
	}
	return path[:i] + ".webp"
}

func (f file) dimsFolder() string {
	return filepath.Join(f.cacheFolder(), "dims")
}

func (f file) dimsFile() string {
	return filepath.Join(f.dimsFolder(), f.base()+".txt")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func modtime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("modtime: %v", err.Error())
	}
	return fi.ModTime(), nil
}

func (f file) modtime() (time.Time, error) {
	return modtime(f.path())
}

func cacheOriginals(opt *options) error {
	// get orig
	// see if cached
	files, err := getOriginals()
	if err != nil {
		return err
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
			err := cacheImage(f, opt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func cacheImage(f file, opt *options) error {
	err := createFolder(f.cacheFolder())
	if err != nil {
		return err
	}

	if !exists(f.dimsFile()) || opt.rerunDims || sourceIsNewer(f, 1600) {
		err := f.createDimsFile()
		if err != nil {
			return err
		}
		fmt.Printf("created dims file %v\n", rel(f.dimsFile()))
	}

	for _, size := range sizes {
		err := createFolder(f.sizeFolder(size))
		if err != nil {
			return err
		}
		rerunKine := false
		if x := strings.Index(f.path(), "/kine/"); opt.rerunFolder == "kine" && x != -1 {
			rerunKine = true
		}
		if !exists(f.cacheFile(size)) || size == opt.rerunSize || rerunKine ||
			isMonth(f.path(), opt.rerunFolder) || sourceIsNewer(f, size) {
			err := f.createCacheFile(size)
			if err != nil {
				return err
			}
			continue
		}
		fmt.Printf("skipping %v\t(%v)\talready cached\n", f.rel(), size)
	}

	return nil
}

func deleteCached() error {
	cacheFiles, err := getCached()
	if err != nil {
		return err
	}
	for _, cacheFile := range cacheFiles {
		if !exists(cacheFile.originalPath()) {
			err = os.Remove(cacheFile.path())
			if err != nil {
				fmt.Printf("unsuccesful in deleting %v\n", cacheFile.rel())
				continue
			}
			p := cacheFile.pathWebP()
			err = os.Remove(p)
			if err != nil {
				fmt.Printf("unsuccesful in deleting %v\n", rel(p))
				continue
			}
			fmt.Printf("deleted -- source gone %v\n", rel(p))
		}
	}
	return nil
}

func (f file) createDimsFile() error {
	if !exists(f.dimsFolder()) {
		err := os.Mkdir(f.dimsFolder(), 0755)
		if err != nil {
			return err
		}
	}
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	err := mw.ReadImage(f.path())
	if err != nil {
		panic(err)
	}

	w := mw.GetImageWidth()
	h := mw.GetImageHeight()

	return os.WriteFile(f.dimsFile(), []byte(fmt.Sprintf("%dx%d", w, h)), 0644)
}

func sourceIsNewer(f file, size int) bool {
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

func createFolder(path string) error {
	if !exists(path) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			return err
		}
		fmt.Printf("created %v (folder)\n", rel(path))
	}
	return nil
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

func (f file) createCacheFile(size int) error {
	if f.ext() != ".jpg" {
		return fmt.Errorf("caching of non-jpeg files is not supported")
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
		if orientation == "portrait" {
			// height
			mw = mw.TransformImage("", fmt.Sprintf("x%v", size))
		} else {
			// width
			mw = mw.TransformImage("", fmt.Sprintf("%v", size))
		}

		err := mw.SetImageCompressionQuality(90)
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

	out, err := os.Create(f.cacheFile(size))
	if err != nil {
		return err
	}
	defer out.Close()

	err = mw.WriteImageFile(out)
	if err != nil {
		return err
	}

	fmt.Printf("cached \t %v\t(%v)\n", f.rel(), size)

	wmw := mw.Clone()
	err = wmw.SetImageFormat("WEBP")
	if err != nil {
		return err
	}

	/*
		err = wmw.SetOption("webp:lossless", "true")
		if err != nil {
			return err
		}
	*/
	/*
		err = wmw.SetImageCompressionQuality(80)
		if err != nil {
			return err
		}
	*/

	err = wmw.WriteImage(f.cacheFileWebP(size))
	if err != nil {
		return err
	}

	fmt.Printf("cached \t %v\n", rel(f.cacheFileWebP(size)))

	blur, err := os.Create(f.cacheFileBlur(size))
	if err != nil {
		return err
	}
	defer blur.Close()

	if orientation == "portrait" {
		mw = mw.TransformImage("", fmt.Sprintf("x%v", 320))
	} else {
		mw = mw.TransformImage("", fmt.Sprintf("%v", 320))
	}

	// 12 normal
	// 30 superblur
	//    placeholder == black or gray image
	err = mw.BlurImage(0, 12)
	if err != nil {
		return err
	}

	if orientation == "portrait" {
		mw = mw.TransformImage("", fmt.Sprintf("x%v", size))
	} else {
		mw = mw.TransformImage("", fmt.Sprintf("%v", size))
	}

	err = mw.WriteImageFile(blur)
	if err != nil {
		return err
	}

	fmt.Printf("cached \t %v\n", rel(f.cacheFileBlur(size)))

	mw.Destroy()
	return nil
}

func renameImage(f file) (file, error) {
	nn, err := readExifDate(f.path())
	if err != nil {
		return "", err
	}
	nf := file(filepath.Join(f.dir(), nn))

	fmt.Printf("renamed %v to\n-> %v\n", f.base(), nf.base())
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
