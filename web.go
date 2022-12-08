package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// global variables
var _top, _bottom string

// Configuration stores server configuration parameters
type Configuration struct {
	Port      int    `json:"port"`      // server port number
	Base      string `json:"base"`      // server base end-point
	StaticDir string `json:"staticdir"` // location of static directory
	Templates string `json:"templates"`
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
	if Config.Templates == "" {
		Config.Templates = fmt.Sprintf("%s/templates", Config.StaticDir)
	}
	log.Printf("config %+v", Config)
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

	// end-points
	router.HandleFunc(basePath("/stats"), DataHandler).Methods("POST", "GET")

	// static handlers
	for _, dir := range []string{"js", "css", "images"} {
		m := fmt.Sprintf("%s/%s/", Config.Base, dir)
		d := fmt.Sprintf("%s/%s", Config.StaticDir, dir)
		http.Handle(m, http.StripPrefix(m, http.FileServer(http.Dir(d))))
	}

	// home page
	router.HandleFunc(basePath("/"), HomeHandler).Methods("GET")

	return router
}

// helper function to start web server
func server(webConfig string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	err := parseConfig(webConfig)
	if err != nil {
		log.Fatal(err)
	}

	// static files
	var templates Templates
	tmplData := make(map[string]interface{})
	tmplData["Time"] = time.Now()
	tmplData["Version"] = info()
	tmplData["Base"] = Config.Base
	_top = templates.Tmpl(Config.Templates, "top.tmpl", tmplData)
	_bottom = templates.Tmpl(Config.Templates, "bottom.tmpl", tmplData)

	// server details
	addr := fmt.Sprintf(":%d", Config.Port)
	server := &http.Server{
		Addr: addr,
	}
	http.Handle("/", Handlers())
	log.Printf("Starting HTTP server on %s", addr)
	log.Fatal(server.ListenAndServe())
}

// HomeHandler process incoming requests
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	var templates Templates
	tmplData := make(map[string]interface{})
	tmplData["Base"] = Config.Base
	page := templates.Tmpl(Config.Templates, "main.tmpl", tmplData)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(_top + page + _bottom))
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var workflows []string
		if strings.Contains(string(body), "workflows=") {
			// web form
			data := strings.Replace(string(body), "workflows=", "", -1)
			data, _ = url.QueryUnescape(data)
			data = strings.Replace(data, "\n", " ", -1)
			data = strings.Replace(data, "\r", "", -1)
			arr := strings.Split(data, " ")
			for _, w := range arr {
				workflows = append(workflows, strings.Trim(w, " "))
			}
		} else {
			err = json.Unmarshal(body, &workflows)
		}
		log.Println("workflows", workflows)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		out, err = concurrentCheck(workflows)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
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
