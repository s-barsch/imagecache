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

	"gopkg.in/gographics/imagick.v2/imagick"
)

type Options struct {
	RerunFolder string
	RerunSize   int
	RerunDims   bool
	Writer      io.Writer
}

var Writer io.Writer

var validFilename = regexp.MustCompile("^[0-9]{6}_[0-9]{6}[a-z\u00E0-\u00FC-+]*\\.[a-z]+$")
var numFilename = regexp.MustCompile("/img/[0-9]{3,4}/")

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

func CacheImage(f File, opt *Options) error {
	if opt.Writer != nil {
		Writer = opt.Writer
	}
	err := createFolder(f.cacheFolder())
	if err != nil {
		return err
	}

	if !exists(f.dimsFile()) || opt.RerunDims || sourceIsNewer(f, 1600) {
		err := f.createDimsFile()
		if err != nil {
			return err
		}
		Print("created dims file %v", f.dimsFile())
	}

	for _, size := range sizes {
		err := createFolder(f.sizeFolder(size))
		if err != nil {
			return err
		}
		rerunKine := false
		if x := strings.Index(f.path(), "/kine/"); opt.RerunFolder == "kine" && x != -1 {
			rerunKine = true
		}
		if !exists(f.cacheFile(size)) || size == opt.RerunSize || rerunKine ||
			isMonth(f.path(), opt.RerunFolder) || sourceIsNewer(f, size) {
			err := f.createCacheFile(size)
			if err != nil {
				return err
			}
			continue
		}
		Print("skipping: %v -- already cached", f.cacheFile(size))
	}

	return nil
}

func (f File) createCacheFile(size int) error {
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

	p := f.cacheFile(size)
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()

	err = mw.WriteImageFile(out)
	if err != nil {
		return err
	}

	Print("cached: %v", p)

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

	Print("cached: %v", f.cacheFileWebP(size))

	/*
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

		fmt.Printf("cached \t %v\n", cap(f.cacheFileBlur(size)))
	*/

	mw.Destroy()
	return nil
}

func (f File) createDimsFile() error {
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

func createFolder(path string) error {
	if !exists(path) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			return err
		}
		Print("created %v (folder)", path)
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

func (f File) cacheFile(size int) string {
	return filepath.Join(f.sizeFolder(size), f.base())
}

func (f File) cacheFileWebP(size int) string {
	path := f.cacheFile(size)
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		panic("invalid path")
	}
	return path[:i] + ".webp"
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

func Cap(path string) string {
	const data = "/data"
	const l = len(data)
	if i := strings.Index(path, data); i > 0+l {
		return path[i+l:]
	}
	return path
}
