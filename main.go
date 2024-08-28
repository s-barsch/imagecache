package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"g.sacerb.com/imagecache/cache"
)

func main() {
	rerunFolder := flag.String("rerun", "", "which folder should be freshly cached? eg: 17-07. write \"all\" for everything")
	rerunSize := flag.Int("size", 0, "specify if a specific size should be recached")
	rerunDims := flag.Bool("rerunDims", false, "recreate dimension files for all images")
	pathsCfg := flag.String("config", "./paths.cfg", "provide path to paths.cfg")
	flag.Parse()

	opt := &cache.Options{
		RerunFolder: *rerunFolder,
		RerunSize:   *rerunSize,
		RerunDims:   *rerunDims,
	}

	cache.Abort = make(chan os.Signal, 1)
	signal.Notify(cache.Abort, os.Interrupt)

	paths, err := readPaths(*pathsCfg)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = cachePaths(paths, opt)
	fmt.Println(err)
}

func cachePaths(paths []string, opt *cache.Options) error {
	for _, root := range paths {
		err := cache.CacheImages(root, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

func readPaths(pathsCfg string) ([]string, error) {
	b, err := os.ReadFile(pathsCfg)
	if err != nil {
		return nil, fmt.Errorf("provide a paths.cfg")
	}
	lines := strings.Split(string(b), "\n")
	paths := []string{}
	for _, l := range lines {
		if l[0] == '#' {
			continue
		}
		paths = append(paths, strings.TrimSpace(l))
	}
	return paths, nil
}
