package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/gographics/imagick.v2/imagick"
)

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

func rel(path string) string {
	if len(path) > rootl {
		return path[rootl:]
	}
	return path
}
