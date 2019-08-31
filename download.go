package main

import (
	"download/cli"
	"download/conf"
	"download/ui"
	"encoding/json"
	"fmt"
	pb "github.com/alexfedorenchik/multibar"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	TmpExt = ".dld"
)

type WriteCounter struct {
	Total   uint64
	updater pb.ProgressFunc
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.updater(int(wc.Total))
	return n, nil
}

type FileShortInfo struct {
	Path string
	Name string
	Size int64
}

const POOL = 3

func main() {
	var params = cli.Params{}
	params.Load()
	params.Print()
	Run(params)
}

func Run(params cli.Params) {
	config := loadConfig(params)
	source := chooseSource(config)
	resolveParams(&source)
	dirs := resolvePaths(source)
	paths := resolveFiles(dirs)
	files := collectFiles(paths)
	download(files, params)
	time.Sleep(time.Second)
}

func loadConfig(params cli.Params) (configuration conf.Configuration) {
	jsonFile, err := os.Open(params.ConfigFile)
	if err != nil {
		log.Fatalf("ERROR: Unable to open config file due to %v", err)
	}
	defer func() {
		if err := jsonFile.Close(); err != nil {
			log.Printf("ERROR: Unable to close config file due to %v", err)
		}
	}()

	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatalf("ERROR: Unable to read config file due to %v", err)
	}

	err = json.Unmarshal(data, &configuration)
	if err != nil {
		log.Fatalf("ERROR: Unable to parse config file due to %v", err)
	}
	return
}

func chooseSource(configuration conf.Configuration) (res conf.Source) {
	nums := ui.ProcessUserInput(configuration, false)
	res = configuration.Sources[nums[0]-1]
	log.Printf("Source %v is chosen", res.Name)
	return
}

func resolveParams(source *conf.Source) {
	for i, param := range source.Parameters {
		nums := ui.ProcessUserInput(param, true)
		vals := make([]string, len(nums))
		for i, j := range nums {
			vals[i] = param.Values[j-1].Value
		}
		param.Resolve(vals)
		source.Parameters[i] = param
	}
}

func resolvePaths(source conf.Source) []string {
	paths := source.Path
	for _, param := range source.Parameters {
		newPaths := make([]string, 0)
		for _, p := range paths {
			for _, v := range param.GetResolved() {
				s := strings.ReplaceAll(p, "${"+param.Name+"}", v)
				newPaths = append(newPaths, s)
			}
		}
		paths = newPaths
	}
	return paths
}

func resolveFiles(paths []string) (res []string) {
	progressBars, _ := pb.New()
	fmt.Println()
	_, err := progressBars.Printf("Looking for files. Introspecting %v pattern(s).\n", len(paths))
	if err != nil {
		log.Fatalf("Failed to show porgress due to %v", err)
	}
	bar := progressBars.MakeBar(len(paths), "Processing folders: ")
	go progressBars.Listen()
	res = make([]string, 0)
	c := make(chan []string)
	wg := new(sync.WaitGroup)
	bar(0)
	for _, s := range paths {
		wg.Add(1)
		go getFiles(s, c, wg)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < len(paths); i++ {
			res = append(res, <-c...)
			bar(i)
		}
	}()
	wg.Wait()
	bar(len(paths))
	return
}

func getFiles(s string, c chan []string, wg *sync.WaitGroup) {
	defer wg.Done()
	dat, err := filepath.Glob(s)
	if err != nil {
		log.Fatalf("Resulting pattern is %v is invalid. %v", s, err)
	}
	c <- dat
}

func collectFiles(files []string) (res []FileShortInfo) {
	time.Sleep(time.Second)
	fmt.Println()
	progressBars, _ := pb.New()
	_, err := progressBars.Printf("Collecting info for %v files.\n", len(files))
	if err != nil {
		log.Fatalf("Failed to show porgress due to %v", err)
	}
	bar := progressBars.MakeBar(len(files), "Processing files: ")
	go progressBars.Listen()
	bar(0)

	res = make([]FileShortInfo, 0)
	c := make(chan FileShortInfo)
	wg := new(sync.WaitGroup)
	for _, f := range files {
		wg.Add(1)
		go getStats(f, c, wg)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < len(files); i++ {
			res = append(res, <-c)
			bar(i)
		}
	}()
	wg.Wait()
	bar(len(files))
	return
}

func getStats(s string, infos chan FileShortInfo, group *sync.WaitGroup) {
	defer group.Done()
	info, _ := os.Stat(s)
	infos <- FileShortInfo{
		Path: s,
		Name: info.Name(),
		Size: info.Size(),
	}
}

func download(files []FileShortInfo, params cli.Params) {
	time.Sleep(time.Second)
	fmt.Println()
	progressBars, _ := pb.New()
	_, err := progressBars.Println("Downloading files.")
	if err != nil {
		log.Fatalf("Failed to show porgress due to %v", err)
	}
	jobs := make(chan FileShortInfo)
	res := make(chan int)
	wg := new(sync.WaitGroup)

	for w := 0; w < POOL; w++ {
		updater := progressBars.MakeBar(100, "                                                      ")
		go worker(jobs, res, updater, progressBars.Bars[w], params)
	}
	bar := progressBars.MakeBar(len(files), "Total: ")
	go progressBars.Listen()
	bar(0)

	go func() {
		for i := 0; i < len(files); i++ {
			<-res
			bar(i + 1)
			wg.Done()
		}
	}()

	for _, file := range files {
		wg.Add(1)
		jobs <- file
	}
	close(jobs)

	wg.Wait()
}

func worker(jobs <-chan FileShortInfo, res chan<- int, updateFunc pb.ProgressFunc, bar *pb.ProgressBar, params cli.Params) {
	for j := range jobs {
		bar.StartTime = time.Now()
		bar.Total = int(j.Size)
		delta := len(bar.Prepend) - len(j.Name)
		bar.Prepend = j.Name
		bar.Width = bar.Width + delta
		updateFunc(0)
		processFile(j, updateFunc, params)
		res <- 0
	}
	bar.StartTime = time.Now()
	delta := len(bar.Prepend) - len("Done:")
	bar.Prepend = "Done:"
	bar.Width = bar.Width + delta
	bar.Total = 1
	updateFunc(1)
}

func processFile(info FileShortInfo, bar pb.ProgressFunc, params cli.Params) {
	dst := filepath.Join(params.WorkingDir, info.Name+TmpExt)

	hasToRename, err := copyFile(info, bar, params)
	if err != nil {
		log.Fatalf("Failed to flush file %v due to %v", dst, err)
	}

	if hasToRename {
		final := filepath.Join(params.WorkingDir, info.Name+"")
		err := os.Rename(dst, final)
		if err != nil {
			log.Fatalf("ERROR:  Failed to rename file %v due to %v", dst, err)
		}

	}
}

func copyFile(info FileShortInfo, bar pb.ProgressFunc, params cli.Params) (bool, error) {
	if checkDone(info, params) {
		bar(int(info.Size))
		return false, nil
	}

	file, err := os.Open(info.Path)
	if err != nil {
		log.Fatalf("Failed to open file %v due to %v", info.Path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatalf("Failed to close file %v due to %v", info.Path, err)
		}
	}()

	dst := filepath.Join(params.WorkingDir, info.Name+TmpExt)
	destination, err := os.Create(dst)
	if err != nil {
		log.Fatalf("Failed to create file %v due to %v", dst, err)
	}
	defer func() {
		if err := destination.Close(); err != nil {
			log.Fatalf("Failed to close file %v due to %v", dst, err)
		}
	}()

	counter := &WriteCounter{
		Total:   0,
		updater: bar,
	}

	_, err = io.Copy(destination, io.TeeReader(file, counter))
	if err != nil {
		log.Fatalf("Failed to copy file %v due to %v", dst, err)
	}

	return true, destination.Sync()
}

func checkDone(info FileShortInfo, params cli.Params) bool {
	tmp := filepath.Join(params.WorkingDir, info.Name+TmpExt)
	_, err := os.Stat(tmp)
	if !os.IsNotExist(err) {
		if err = os.Remove(tmp); err != nil {
			log.Fatalf("Failed to delete partialy downloaded file %v due to %v", tmp, err)
		}
	}
	dst := filepath.Join(params.WorkingDir, info.Name)
	stat, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return false
	}
	if stat.Size() != info.Size {
		if err = os.Remove(dst); err != nil {
			log.Fatalf("Failed to delete partialy downloaded file %v due to %v", dst, err)
		}
		return false
	}
	return true
}
