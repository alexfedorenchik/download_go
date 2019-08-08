package cli

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

type Params struct {
	ConfigFile string
	WorkingDir string
}

func (it *Params) Load() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	ex, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exPath := filepath.Dir(ex)

	var confFile string
	flag.StringVar(&confFile, "config", filepath.Join(exPath, "download.json"), "Configuration file")

	flag.Parse()

	it.WorkingDir = dir
	it.ConfigFile = confFile
}

func (it *Params) Print() {
	log.Printf("========================================")
	log.Printf("Start params")
	log.Printf("Working dir: %v", it.WorkingDir)
	log.Printf("Config file: %v", it.ConfigFile)
	log.Printf("========================================")
}
