package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	git "gopkg.in/src-d/go-git.v4"
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
		// if matchList(pathName, includes) || !matchList(pathName, excludes) {
		// 	fmt.Println()
		// }
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

func localInit() {
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
}

func getRemote(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return "", err
	}
	var remoteURLs []string
	for _, i := range remotes {
		remoteURLs = append(remoteURLs, i.Config().URLs...)
	}
	if len(remoteURLs) > 0 {
		return remoteURLs[0], nil
	}
	return "", errors.New("No remotes found")
}

func main() {
	localInit()
	gitDirs := findGitDirs(searchDir, includes, excludes)
	pathRemoteMap := make(map[string]string)
	for _, dir := range gitDirs {
		remote, err := getRemote(dir)
		if err != nil {
			// fmt.Println(err)
			continue
		}
		pathRemoteMap[dir] = remote
	}
	for key, val := range pathRemoteMap {
		fmt.Printf("%s: %s\n", key, val)
	}
	// fmt.Println(pathRemoteMap)
}
