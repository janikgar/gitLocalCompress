package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/karrick/godirwalk"

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

func findGitDirs(dirName string, includes listFlags, excludes listFlags) ([]string, error) {
	dirName = filepath.ToSlash(dirName)
	var foundDirs []string
	err := godirwalk.Walk(dirName, &godirwalk.Options{
		Unsorted: true,
		Callback: func(pathName string, dirent *godirwalk.Dirent) error {
			if (matchList(pathName, includes) || !matchList(pathName, excludes)) && dirent.IsDir() && dirent.Name() == ".git" {
				foundDirs = append(foundDirs, filepath.Dir(pathName))
				fmt.Printf(".")
			}
			return nil
		},
	})
	if err != nil {
		return []string{}, err
	}
	fmt.Printf("|\n")
	return foundDirs, nil
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

func getAllRemotes(gitDirs []string) map[string]string {
	pathRemoteMap := make(map[string]string)
	for _, dir := range gitDirs {
		remote, err := getRemote(dir)
		if err != nil {
			// fmt.Println(err)
			continue
		}
		if remote[len(remote)-4:] == ".git" {
			pathRemoteMap[dir] = remote
			fmt.Printf(".")
		}
	}
	fmt.Printf("|\n")
	return pathRemoteMap
}

func mockPlainClone(inputURL <-chan string, outputURL chan<- string, failedURL chan<- string, errors chan<- error) {
	// func mockPlainClone(gitURL string) (string, string, error) {
	gitURL := <-inputURL
	tempDir := filepath.Join(os.TempDir(), filepath.Base(gitURL))
	defer os.RemoveAll(tempDir)
	_, err := git.PlainClone(tempDir, true, &git.CloneOptions{URL: gitURL})
	if err != nil {
		outputURL <- ""
		fmt.Print("1")
		failedURL <- gitURL
		errors <- err
		return
		// return "", gitURL, err
	}
	fmt.Print("0")
	outputURL <- gitURL
	failedURL <- ""
	errors <- nil
	return
	// return gitURL, "", nil
}

func handleRemotes(pathRemoteMap map[string]string) {
	var wg sync.WaitGroup
	remoteURL := make(chan string, len(pathRemoteMap))
	clonedURL := make(chan string)
	failedURL := make(chan string)
	errors := make(chan error)
	for _, k := range pathRemoteMap {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mockPlainClone(remoteURL, clonedURL, failedURL, errors)
		}()
		remoteURL <- k
	}
	var allClones []string
	var allFailed []string
	var allErrors []error
	for {
		allClones = append(allClones, <-clonedURL)
		allFailed = append(allFailed, <-failedURL)
		allErrors = append(allErrors, <-errors)
		<-remoteURL
		if len(<-remoteURL) == 0 {
			break
		}
	}
	wg.Wait()
}

func main() {
	localInit()
	gitDirs, err := findGitDirs(searchDir, includes, excludes)
	if err != nil {
		fmt.Println(err)
	}
	pathRemoteMap := getAllRemotes(gitDirs)
	handleRemotes(pathRemoteMap)
}
