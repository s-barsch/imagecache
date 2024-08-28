package main

// TODO: there is an issue if one image of an already cached series is deleted. series doesnt show up.

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
)

// var root = "/srv/rg-s/st/data"
var root = "/Volumes/External/srv/rg-s/st/data/kine"

// var root = "/Volumes/External/srv/rg-s/st/data/graph"
var rootl = len(root)

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

// var validFilename = regexp.MustCompile("^[0-9]+_[0-9]+.jpg$")
var validFilename = regexp.MustCompile("^[0-9]{6}_[0-9]{6}[a-z\u00E0-\u00FC-+]*\\.[a-z]+$")

func cacheImages(opt *options) error {
	err := cacheOriginals(opt)
	if err != nil {
		return err
	}

	return deleteCached()
}
