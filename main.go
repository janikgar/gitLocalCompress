package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// var excludes = []string{
// 	"/Users/janik/Documents/Programming/go/src",
// 	"/Users/janik/Documents/Programming/go/pkg/dep",
// }

// var includes = []string{
// 	"/Users/janik/Documents/Programming/go/src/github.com/janikgar",
// }

type listFlags []string

type inputPatterns struct {
	IncludePatterns listFlags `json:"include"`
	ExcludePatterns listFlags `json:"exclude"`
	SearchPattern   string    `json:"searchDir"`
}

var (
	excludes   listFlags
	includes   listFlags
	configFile string
	searchDir  string
)

// Stringer for array flags (i.e., flags defined more than once)
func (i *listFlags) String() string {
	return fmt.Sprint(*i)
}

// Setter for array flags (i.e., flags defined more than once)
func (i *listFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func getFlagsFromConfig() {
	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err)
	}
	var configContents inputPatterns
	if err = json.Unmarshal(config, &configContents); err != nil {
		fmt.Println(err)
	}
	includes = configContents.IncludePatterns
	excludes = configContents.ExcludePatterns
	searchDir = configContents.SearchPattern
}

func getFlags() {
	flag.StringVar(&configFile, "config", "", "filename of JSON file with includes, excludes, and searchdir")
	flag.Var(&excludes, "exclude", "directory patterns to exclude")
	flag.Var(&includes, "include", "directory patterns to include (even if excluded in blacklist)")
	// searchDir = flag.Arg(0)
}

func matchList(pathName string, list []string) bool {
	for _, match := range list {
		// fmt.Printf("path: %s\nmatch: %s\n\n", strings.ReplaceAll(pathName, "\\", "/"), match)
		if strings.Contains(strings.ReplaceAll(pathName, "\\", "/"), match) {
			return true
		}
	}
	return false
}

func findGitDirs(dirName string, includes listFlags, excludes listFlags) []string {
	dirName = filepath.ToSlash(dirName)
	var foundDirs []string
	err := filepath.Walk(dirName, func(pathName string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if (matchList(pathName, includes) || !matchList(pathName, excludes)) && fileInfo.IsDir() && fileInfo.Name() == ".git" {
			foundDirs = append(foundDirs, filepath.Dir(pathName))
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	return foundDirs
}

func main() {
	getFlags()
	flag.Parse()
	getFlagsFromConfig()
	if searchDir == "" {
		searchDir = flag.Arg(0)
	}
	if flag.NArg() < 1 && searchDir == "" {
		flag.Usage()
		os.Exit(1)
	}
	gitDirs := findGitDirs(searchDir, includes, excludes)
	for _, dir := range gitDirs {
		fmt.Println(dir)
	}
}
