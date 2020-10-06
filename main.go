package main

// TODO: there is an issue if one image of an already cached series is deleted. series doesnt show up.

import (
	//"github.com/nfnt/resize"
	//"github.com/disintegration/imaging"
	//"image"
	//"image/jpeg"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	//"strings"
	"time"
	//"rd"
	"log"
	"flag"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"gopkg.in/gographics/imagick.v2/imagick"
	"math"
	"os/signal"
	"regexp"
)

var root = "/srv/rg-s/st/data"
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

//var validFilename = regexp.MustCompile("^[0-9]+_[0-9]+.jpg$")
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

func (f file) cacheFile(size int) string {
	return filepath.Join(f.sizeFolder(size), f.base())
}

func (f file) dimsFolder() string {
	return filepath.Join(f.cacheFolder(), "dims")
}

func (f file) dimsFile() string {
	return filepath.Join(f.dimsFolder(), f.base()+".txt")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
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
		if !exists(f.cacheFile(size)) || size == opt.rerunSize ||
			f.folder() == opt.rerunFolder || sourceIsNewer(f, size) {
			err := f.createCacheFile(size)
			if err != nil {
				return err
			}
			fmt.Printf("cached \t %v\t(%v)\n", f.rel(), size)

			//fmt.Printf("cached %v ->\n       %v \n", f.rel(), rel(f.cacheFile(size)))
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
			fmt.Printf("deleted -- source gone %v\n", cacheFile.rel())
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

	return ioutil.WriteFile(f.dimsFile(), []byte(fmt.Sprintf("%dx%d", w, h)), 0644)
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

func (f file) createCacheFile(size int) error {
	if f.ext() != ".jpg" {
		return fmt.Errorf("caching of non-jpeg files is not supported.")
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
			mw = mw.TransformImage("", fmt.Sprintf("x%v", size))
		} else {
			mw = mw.TransformImage("", fmt.Sprintf("%v", size))
		}

		err := mw.SetImageCompressionQuality(90)
		if err != nil {
		log.Println("path: %v", f.path())
			return err
		}

		// dont sharpen nexus images with image ratio 4:3
		if math.Trunc((float64(w)/float64(h))*100) == 133 ||
			math.Trunc((float64(h)/float64(w))*100) == 133 {
			err = mw.SharpenImage(0, 0.5)
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

/*
func renameImage(f file) (file, error) {
	// TODO: adapt for formats with and without nano seconds
	if len(f.Name()) < len("IMG_20160102_150405.jpg") {
		return nil, fmt.Errorf("could not rename image: unexpected filename. %v", f.Name())
	}
	println(f.Name())
	nn := f.Name()[6:6+13] + f.Ext()
	nn = strings.Replace(nn, "_24", "_00", -1)
	nf := &File{
		Path: filepath.Join(f.Dir(), nn),
	}
	fmt.Printf("renamed %v\n", f.Path)
	return nf, os.Rename(f.Abs(), nf.Abs())
}
*/

// found this on the internetz and modified it.
func round(val float64) uint {
	var roundn float64
	roundOn := 0.5
	places := 0
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		roundn = math.Ceil(digit)
	} else {
		roundn = math.Floor(digit)
	}
	newVal := roundn / pow
	return uint(newVal)
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

/*
// log file must switch places
// every jpg file must be indexed.
func cacheImages(rerun string, rerunSize string) error {
	fl, err := getFileList()
	if err != nil {
		return err
	}
	logf, err := readLog(logfile)
	if err != nil {
		fmt.Println(err)
	}
	cerrs := []error{}
	// cached image files
	m := map[string]time.Time{}
	for _, f := range fl {
		// see if the image has already been cached.
		if logf != nil {
			if ts := logf[f.Path]; ts != "" {
				delete(logf, f.Path)
				// check if caching can be skipped.
				tstime, err  := time.Parse(time.UnixDate, ts)
				if err != nil {
					return err
				}
				fi, err := os.Stat(f.Abs())
				if err != nil {
					return err
				}
				// see if modification time is newer than timestamp from logfile.
				if tstime.Unix() > fi.ModTime().Unix() && f.Folder() != rerun && rerun != "all" {
					fmt.Printf("skipping %v because its already cached\n", f.Path)
					m[f.Path] = tstime
					continue
				}
			}
		}
		if f.IsDir() {
			//if strings.Contains(f.Path, "_") {
				err := cacheDir(f.Path)
				if err != nil {
					return err
				}
				m[f.Path] = time.Now()
				continue
			//}
			continue
		}
		// IMG_20161201_240101999.jpg
		if !validFilename.MatchString(f.Name()) {
			f, err = renameImage(f)
			if err != nil {
				return err
			}
		}
		err = createFolders(f.Path[:6])
		if err != nil {
			return err
		}
		err = cacheImage(f, rerunSize)
		if err != nil {
			cerrs = append(cerrs, err)
			fmt.Println(err)
			continue
		}
		m[f.Path] = time.Now()
	}
	for i, e := range cerrs {
		if i == 0 {
			fmt.Println("* ERRORS *")
		}
		fmt.Println(e)
	}
	err = writecache(m)
	if err != nil {
		log.Println(err)
	}
	err = diffcache(logf)
	if err != nil {
		log.Println(err)
	}
	return nil
}

func createFolders(p string) error {
	_, err := os.Stat(p)
	if err == nil {
		return nil
	}
	for size, _ := range wsizes {
		err = os.MkdirAll(filepath.Join(root, p, "cache", size), 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffcache(m map[string]string) error {
	for path, _ := range m {
		for size, _ := range wsizes {
			err := os.RemoveAll(filepath.Join(root, path[:6], "cache", size, path[6:]))
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("removed %v (%v) cause its source is gone\n", path, size)
		}
	}
	return nil
}
*/

/*
// see if any files have been deleted from "source" folder, delete all cached copies.
func diffcache(m map[string]time.Time) error {
	for size, _ := range wsizes {
		l, err := readdir(filepath.Join(cache, size), "")
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, f := range l {
			if len(f.Path) < 6 {
				fmt.Printf("err here")
			}
			month := f.Path[:4] // 1609
			p := fmt.Sprintf("/%v-%v/%v", month[:2], month[2:], f.Path)
			if _, ok := m[p]; !ok {
				err = os.RemoveAll(filepath.Join(root, f.Path[:6], size, f.Path[6:]))
				if err != nil {
					//fmt.Println(err)
					return err
				}
				fmt.Printf("removed /%v/%v cause its source is gone\n", size, f.Path)
			}
		}
	}
	return nil
}

func writecache(m map[string]time.Time) error {
	// write cache.fmt.file
	var buf bytes.Buffer
	for k, v := range m {
		_, err := buf.WriteString(fmt.Sprintf("%v: %v\n", k, v.Format(time.UnixDate)))
		if err != nil {
			return err
		}
	}
	err := ioutil.WriteFile(root + "/cache.log", buf.Bytes(), 0666)
	if err != nil {
		return err
	}
	return nil
}
*/
/*
func cacheDir(name string) error {
	for size, _ := range wsizes {
		// cutting the month /16-11 (len 6)
		if len(name) < 6 {
			return fmt.Errorf("dir path too short")
		}
		cp := filepath.Join(root, name[:6], "cache", size, name[6:])
		fi, err := os.Stat(cp)
		if err == nil && fi.IsDir() {
			// already exists
			fmt.Println("cachedir, already exists: ", cp)
			return nil
		}
		err = os.Mkdir(cp, 0755)
		if err != nil {
			return err
		}
		fmt.Printf("created dir %v\n", cp)
		//fmt.Printf("created dir /%v/%v\n", size, name)
	}
	return nil
}
*/
