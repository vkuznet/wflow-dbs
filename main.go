package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

// version of the code
var gitVersion string

// Info function returns version string of the server
func info() string {
	goVersion := runtime.Version()
	tstamp := time.Now().Format("2006-02-01")
	return fmt.Sprintf("dbs2go git=%s go=%s date=%s", gitVersion, goVersion, tstamp)
}

func main() {
	var workflow string
	flag.StringVar(&workflow, "workflow", "workflow.json", "dbs2go workflow file")
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "Show verbose")
	var version bool
	flag.BoolVar(&version, "version", false, "Show version")
	flag.Parse()
	if version {
		fmt.Println(info())
		os.Exit(0)

	}
	check(workflow, verbose)
}

// Record represents output record from checker
type Record struct {
	Workflow           string
	TotalInputLumis    int
	InputDataset       string
	OutputDataset      string
	InputDatasetLumis  int
	OutputDatasetLumis int
}

func check(workflow string, verbose bool) {
	rec, err := callReqMgr(workflow, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to get ReqMgr data for %s, %v", workflow, err)
		os.Exit(1)
	}

	// extract from JSON TotalInputLumis, InputDataset, and list of OutputDatasets
	input := rec.InputDataset
	durl := fmt.Sprintf("https://cmsweb.cern.ch/dbs/prod/global/DBSReader/filesummaries?dataset=%s", input)
	dbsInputRec, err := dbsCall(durl, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to get DBS data for %s, %v", input, err)
		os.Exit(1)
	}
	var out []Record
	for _, output := range rec.OutputDatasets {
		durl = fmt.Sprintf("https://cmsweb.cern.ch/dbs/prod/global/DBSReader/filesummaries?dataset=%s", output)
		dbsOutputRec, err := dbsCall(durl, verbose)
		if err != nil {
			fmt.Printf("ERROR: unable to get DBS data for %s, %v", output, err)
			os.Exit(1)
		}
		rec := Record{
			Workflow:           workflow,
			TotalInputLumis:    rec.TotalInputLumis,
			InputDataset:       input,
			OutputDataset:      output,
			InputDatasetLumis:  dbsInputRec.NumLumis,
			OutputDatasetLumis: dbsOutputRec.NumLumis,
		}
		out = append(out, rec)
	}
	// construct output JSON
	data, err := json.MarshalIndent(out, "", "   ")
	if err == nil {
		fmt.Println(string(data))
	}
}

// DbsRecord represents filesummaries record we need to parse
type DbsRecord struct {
	NumLumis int `json:"num_lumi"`
}

// helper function to perform dbs call
func dbsCall(rurl string, verbose bool) (*DbsRecord, error) {
	req, err := http.NewRequest("GET", rurl, nil)
	if verbose {
		log.Println("dbs call", rurl)
	}
	req.Header.Add("Accept", "application/json")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	if err != nil {
		if verbose {
			log.Println("dbsCall client.Do", err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	var records []DbsRecord
	if err != nil {
		if verbose {
			log.Println("dbsCall io.ReadAll", err)
		}
		return nil, err
	}
	err = json.Unmarshal(data, &records)
	if err != nil {
		if verbose {
			log.Println("dbsCall json.Unmarshal", err)
		}
		return nil, err
	}
	rec := records[0]
	return &rec, nil
}

// WorkflowRecord represent reqmgr map record
type WorkflowRecord map[string]ReqMgrRecord

// ResultRecord represents reqmgr result record
type ResultRecord struct {
	Result []WorkflowRecord
}

// ReqMgrRecord represents subset of reqmgr record we need to parse
type ReqMgrRecord struct {
	InputDataset    string
	OutputDatasets  []string
	TotalInputLumis int
}

// helper function to make call to reqmgr service
func callReqMgr(workflow string, verbose bool) (*ReqMgrRecord, error) {
	// get JSON from reqmgr2 via
	rurl := fmt.Sprintf("https://cmsweb.cern.ch/reqmgr2/data/request?name=%s", workflow)
	if verbose {
		log.Println("rurl", rurl)
	}
	req, err := http.NewRequest("GET", rurl, nil)
	req.Header.Add("Accept", "application/json")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	if err != nil {
		if verbose {
			log.Println("callReqMgr client.Do", err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	var rec ResultRecord
	if err != nil {
		if verbose {
			log.Println("callReqMgr io.ReadAll", err)
		}
		return nil, err
	}
	if verbose {
		log.Println("ReqMgr2 data", string(data))
	}
	err = json.Unmarshal(data, &rec)
	if err != nil {
		if verbose {
			log.Println("callReqMgr json.Unmarshal", err)
		}
		return nil, err
	}
	for _, wrec := range rec.Result {
		for w, r := range wrec {
			if w == workflow {
				return &r, nil
			}
		}
	}
	return nil, errors.New("unable to get ReqMgr record")
}
