package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// version of the code
var gitVersion string

// Info function returns version string of the server
func info() string {
	goVersion := runtime.Version()
	tstamp := time.Now().Format("2006-02-01")
	return fmt.Sprintf("wflow-dbs git=%s go=%s date=%s", gitVersion, goVersion, tstamp)
}

func main() {
	var webConfig string
	flag.StringVar(&webConfig, "webConfig", "", "web server configuration file")
	var workflow string
	flag.StringVar(&workflow, "workflow", "workflow.json", "workflow file")
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "Show verbose")
	var version bool
	flag.BoolVar(&version, "version", false, "Show version")
	flag.Parse()
	if version {
		fmt.Println(info())
		os.Exit(0)

	}
	if webConfig == "" {
		wflows := strings.Split(workflow, ",")
		var out []Record
		var err error
		if len(wflows) == 1 {
			out, err = check(workflow, verbose)
		} else {
			out, err = concurrentCheck(wflows)
		}
		if err != nil {
			log.Fatal(err)
		}
		// construct output JSON
		data, err := json.MarshalIndent(out, "", "   ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(data))
		return
	}
	server(webConfig)
}
