package main

import (
	"download/cli"
	"download/conf"
	"download/ui"
	"encoding/json"
	"fmt"
	pb "github.com/sethgrid/multibar"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type WriteCounter struct {
	Total uint64
	updater pb.ProgressFunc
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.updater(int(wc.Total))
	return n, nil
}

const POOL = 3

func main() {
	var params = cli.Params{}
	params.Load()
	params.Print()
	//	params.ConfigFile = "/Users/afedorenchik/go/bin/download.json"
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
			vals[i] = string(param.Values[j-1])
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
	_, err := progressBars.Printf("Looking for files in %v folders.\n", len(paths))
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
	//defer fmt.Printf("%v files found\n", len(res))
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

func collectFiles(files []string) (res []*os.File) {
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

	res = make([]*os.File, 0)
	c := make(chan *os.File)
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
	//defer fmt.Printf("%v files are scheduled for download\n", len(res))
	return
}

func getStats(s string, infos chan *os.File, group *sync.WaitGroup) {
	defer group.Done()
	info, err := os.Open(s)
	if err != nil {
		log.Fatalf("Failed to collect file %v info due to %v", s, err)
	}
	infos <- info
}

func download(files []*os.File, params cli.Params) {
	time.Sleep(time.Second)
	fmt.Println()
	progressBars, _ := pb.New()
	_, err := progressBars.Println("Downloading files.")
	if err != nil {
		log.Fatalf("Failed to show porgress due to %v", err)
	}
	jobs := make(chan *os.File)
	res := make(chan int)
	wg := new(sync.WaitGroup)

	for w := 0; w < POOL; w++ {
		updater := progressBars.MakeBar(100, "                                                      ")
		go worker(jobs, res, updater, progressBars.Bars[w], params)
	}
	bar := progressBars.MakeBar(len(files), "Total: ")
	//bar(0)
	go progressBars.Listen()
	bar(0)

	go func() {
		for i := 0; i < len(files); i++ {
			<-res
			bar(i+1)
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

func worker(jobs <-chan *os.File, res chan<- int, updateFunc pb.ProgressFunc, bar *pb.ProgressBar, params cli.Params) {
	for j := range jobs {
		fi, _ := j.Stat()
		bar.StartTime = time.Now()
		bar.Total = int(fi.Size())
		delta := len(bar.Prepend) - len(fi.Name())
		bar.Prepend = fi.Name()
		bar.Width = bar.Width + delta
		updateFunc(0)
		processFile(j, fi, updateFunc, params)
		res <- 0
	}
	bar.StartTime = time.Now()
	delta := len(bar.Prepend) - len("Done:")
	bar.Prepend = "Done:"
	bar.Width = bar.Width + delta
	bar.Total = 1
	updateFunc(1)
}

func processFile(file *os.File, info os.FileInfo, bar pb.ProgressFunc, params cli.Params) {
	dst := filepath.Join(params.WorkingDir, info.Name() + ".tmp")
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file %v due to %v", dst, err)
		}
	}()

	destination, err := os.Create(dst)
	if err != nil {
		log.Fatalf("Failed to create file %v due to %v", dst, err)
	}
	defer func() {
		if err := destination.Close(); err != nil {
			log.Printf("Failed to close file %v due to %v", dst, err)
		}
	}()

	done := checkDone(info, params)

	if !done {
		counter := &WriteCounter{
			Total:0,
			updater:bar,
		}

		_, err = io.Copy(destination, io.TeeReader(file, counter))
		if err != nil {
			log.Fatalf("Failed to copy file %v due to %v", dst, err)
		}

		err = destination.Sync()
		if err != nil {
			log.Fatalf("Failed to flush file %v due to %v", dst, err)
		}

		final := filepath.Join(params.WorkingDir, info.Name() + "")
		err := os.Rename(dst, final)
		if err != nil {
			log.Printf("ERROR:  Failed to rename file %v due to %v", final, err)
		}

	} else {
		bar(int(info.Size()))
	}
}

func checkDone(info os.FileInfo, params cli.Params) bool {
	tmp := filepath.Join(params.WorkingDir, info.Name() + ".tmp")
	_, err := os.Stat(tmp)
	if !os.IsNotExist(err) {
		if err = os.Remove(tmp); err != nil {
			log.Fatalf("Failed to delete partialy downloaded file %v due to %v", tmp, err)
		}
	}

	dst := filepath.Join(params.WorkingDir, info.Name())
	stat, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return false
	}
	if stat.Size() != info.Size() {
		if err = os.Remove(dst); err != nil {
			log.Fatalf("Failed to delete partialy downloaded file %v due to %v", dst, err)
		}
		return false
	}

	return true
}

//func fake(val interface{}) {}
//
//func debug(val interface{}) {
//	log.Print(val)
//}
//
//func debuglist(vals []*os.File) {
//	for i, val := range vals {
//		v, _ := val.Stat()
//		log.Printf("%3v. %v", i+1, v)
//	}
//}

