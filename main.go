package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/karrick/godirwalk"

	git "github.com/go-git/go-git/v5"
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
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "  Searching given directories: "
	s.Color("green", "bold")
	s.Start()
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
	s.Stop()
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

// func getRemote(path string) (string, error) {
// 	repo, err := git.PlainOpen(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	remotes, err := repo.Remotes()
// 	if err != nil {
// 		return "", err
// 	}
// 	var remoteURLs []string
// 	for _, i := range remotes {
// 		remoteURLs = append(remoteURLs, i.Config().URLs...)
// 	}
// 	if len(remoteURLs) > 0 {
// 		return remoteURLs[0], nil
// 	}
// 	return "", errors.New("No remotes found")
// }

// func getAllRemotes(gitDirs []string) map[string]string {
// 	pathRemoteMap := make(map[string]string)
// 	for _, dir := range gitDirs {
// 		remote, err := getRemote(dir)
// 		if err != nil {
// 			continue
// 		}
// 		if remote[len(remote)-4:] == ".git" {
// 			pathRemoteMap[dir] = remote
// 		}
// 	}
// 	return pathRemoteMap
// }

func queueCloneDirs(dirs []string, dirChan chan string) {
	for _, dir := range dirs {
		dirChan <- dir
	}
	close(dirChan)
}

func tarGz(tempDir string) {
	// var ioWriter io.Writer
	tarBytes := new(bytes.Buffer)

	// gzWriter := gzip.NewWriter(ioWriter)
	// defer gzWriter.Close()

	tarWriter := tar.NewWriter(tarBytes)
	defer tarWriter.Close()

	err := godirwalk.Walk(tempDir, &godirwalk.Options{
		Unsorted: true,
		Callback: func(pathName string, dirent *godirwalk.Dirent) error {
			if !dirent.IsRegular() {
				return nil
			}
			stat, err := os.Stat(pathName)
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(stat, stat.Name())
			if err != nil {
				return err
			}
			if filepath.IsAbs(pathName) {
				header.Name = strings.TrimPrefix(strings.Replace(pathName, tempDir, "", -1), string(filepath.Separator))
			}
			if stat.IsDir() {
				header.Size = 0
			}
			// fmt.Printf("Name: %s\nSize: %d\nMode: %s\nTime: %s\nDir?: %t\n", header.Name, header.Size, stat.Mode().String(), stat.ModTime(), stat.IsDir())
			// fmt.Printf("%s\n%d\n%s\n%s\n%t\n", header.Name, header.Size, header.FileInfo().Mode().String(), header.ModTime.Format("20060102"), header.FileInfo().IsDir())
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if stat.IsDir() {
				return nil
			}
			fi, err := os.Open(pathName)
			if err != nil {
				return err
			}
			defer fi.Close()
			if _, err := io.Copy(tarWriter, fi); err != nil {
				return err
			}
			if err = tarWriter.Flush(); err != nil {
				return err
			}
			// if err = gzWriter.Flush(); err != nil {
			// 	return err
			// }
			// gzWriter.Flush()
			return nil
		},
		ErrorCallback: func(pathname string, err error) godirwalk.ErrorAction {
			fmt.Printf("ERROR: %s: %s\n", pathname, err.Error())
			return godirwalk.SkipNode
		},
	})
	if err != nil {
		fmt.Println(err)
	}
	tempFileName := fmt.Sprintf("%s.tar", tempDir)
	fmt.Printf("Writing file to %s...\n", tempFileName)
	time.Sleep(time.Second)
	tempFile, err := os.Create(tempFileName)
	if err != nil {
		fmt.Println(err)
	}
	if _, err = io.Copy(tempFile, tarBytes); err != nil {
		fmt.Println(err)
	}
}

func coordinate(gitDirs []string, dirChan chan string, cloneDone chan int, tarGzDone chan int) {
	responses := make(chan cloneResponse, len(gitDirs))
	for {
		select {
		case repo, ok := <-dirChan:
			if !ok {
				cloneDone <- 1
			}
			go cloneRepos(repo, responses)
		case response, ok := <-responses:
			if !ok {
				tarGzDone <- 1
			}
			if response.success {
				go tarGz(response.tempDir)
			}
		}
	}
}

func cloneRepos(repo string, responses chan<- cloneResponse) {
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("%s.git", filepath.Base(repo)))
	_, err := git.PlainClone(tempDir, true, &git.CloneOptions{URL: repo})
	if err != nil && err.Error() != "repository already exists" {
		responses <- cloneResponse{
			repo:    repo,
			tempDir: tempDir,
			success: false,
			err:     err,
		}
	}
	fmt.Printf("Cloning %s...\n", filepath.Base(repo))
	responses <- cloneResponse{
		repo:    repo,
		tempDir: tempDir,
		success: true,
		err:     nil,
	}
}

type cloneResponse struct {
	repo    string
	tempDir string
	success bool
	err     error
}

func main() {
	gitDirs, err := findGitDirs(searchDir, includes, excludes)
	if err != nil {
		fmt.Println(err)
	}
	dirChan := make(chan string)
	cloneDone := make(chan int)
	tarGzDone := make(chan int)

	// go fmt.Printf("Directories: %d\n", cap(dirChan))
	go queueCloneDirs(gitDirs, dirChan)

	// for _, dir := range gitDirs {
	// 	dirChan <- dir
	// }
	// defer close(dirChan)

	go coordinate(gitDirs, dirChan, cloneDone, tarGzDone)
	<-dirChan
	<-tarGzDone
	<-cloneDone
}
