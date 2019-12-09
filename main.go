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

	"github.com/karrick/godirwalk"

	git "gopkg.in/src-d/go-git.v4"
)

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
}

func matchList(pathName string, list []string) bool {
	for _, match := range list {
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
			}
			return nil
		},
	})
	if err != nil {
		return []string{}, err
	}
	return foundDirs, nil
}

func init() {
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
			continue
		}
		if remote[len(remote)-4:] == ".git" {
			pathRemoteMap[dir] = remote
		}
	}
	return pathRemoteMap
}

func queueCloneDirs(dirs []string, dirChan chan string) {
	fmt.Println("started queueing")
	for _, dir := range dirs {
		dirChan <- dir
	}
	close(dirChan)
}

func coordinate(dirChan chan string, done chan int) {
	responses := make(chan cloneResponse)
	for {
		select {
		case repo, ok := <-dirChan:
			if !ok {
				done <- 1
			}
			go pullRepos(repo, responses)
		case response := <-responses:
			if response.success {
				fmt.Println(response.repo)
			} else {
				fmt.Println(response.err)
			}
		}
	}
}

func pullRepos(repo string, responses chan cloneResponse) {
	tempDir := filepath.Join(os.TempDir(), filepath.Base(repo))
	_, err := git.PlainClone(tempDir, true, &git.CloneOptions{URL: repo})
	if err != nil && err.Error() != "repository already exists" {
		responses <- cloneResponse{
			repo:    repo,
			success: false,
			err:     err,
		}
	}
	responses <- cloneResponse{
		repo:    repo,
		success: true,
		err:     nil,
	}
}

type cloneResponse struct {
	repo    string
	success bool
	err     error
}

func main() {
	// localInit()

	gitDirs, err := findGitDirs(searchDir, includes, excludes)
	if err != nil {
		fmt.Println(err)
	}
	dirChan := make(chan string)
	done := make(chan int)

	go queueCloneDirs(gitDirs, dirChan)
	go coordinate(dirChan, done)
	<-done
}
