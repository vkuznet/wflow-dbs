package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

// Configuration stores server configuration parameters
type Configuration struct {
	Port int    `json:"port"` // server port number
	Base string `json:"base"` // server base end-point
}

// Config variable represents configuration object
var Config Configuration

// helper function to parse server configuration file
func parseConfig(configFile string) error {
	data, err := os.ReadFile(filepath.Clean(configFile))
	if err != nil {
		log.Println("Unable to read", err)
		return err
	}
	err = json.Unmarshal(data, &Config)
	return nil
}

// helper function to get base path
func basePath(api string) string {
	base := Config.Base
	if base != "" {
		if strings.HasPrefix(api, "/") {
			api = strings.Replace(api, "/", "", 1)
		}
		if strings.HasPrefix(base, "/") {
			return fmt.Sprintf("%s/%s", base, api)
		}
		return fmt.Sprintf("/%s/%s", base, api)
	}
	return api
}

// Handlers provides helper function to setup all HTTP routes
func Handlers() *mux.Router {
	router := mux.NewRouter()
	router.StrictSlash(true) // to allow /route and /route/ end-points
	router.HandleFunc(basePath("/stats"), DataHandler).Methods("POST", "GET")
	return router
}

// helper function to start web server
func server(webConfig string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	err := parseConfig(webConfig)
	if err != nil {
		log.Fatal(err)
	}
	addr := fmt.Sprintf(":%d", Config.Port)
	server := &http.Server{
		Addr: addr,
	}
	http.Handle("/", Handlers())
	log.Printf("Starting HTTP server on %s", addr)
	log.Fatal(server.ListenAndServe())
}

// DataHandler process incoming requests
func DataHandler(w http.ResponseWriter, r *http.Request) {
	var out []Record
	var err error
	if r.Method == "GET" {
		var workflow string
		for k, values := range r.URL.Query() {
			if k == "workflow" {
				workflow = values[0]
			}
		}
		if workflow == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		out, err = check(workflow, false)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if r.Method == "POST" {
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		var workflows []string
		err = decoder.Decode(&workflows)
		log.Println("workflows", workflows)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, wflow := range workflows {
			results, err := check(wflow, false)
			if err == nil {
				for _, r := range results {
					out = append(out, r)
				}
			}
		}
	}
	// construct output JSON
	data, err := json.MarshalIndent(out, "", "   ")
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// set HTTP headers and data output
	w.Header().Add("Content-Type", "application/json")
	w.Write(data)
}
