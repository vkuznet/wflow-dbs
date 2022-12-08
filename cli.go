package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Record represents output record from checker
type Record struct {
	Workflow        string
	TotalInputLumis int
	InputDataset    string
	OutputDataset   string
	InputStats      DBSRecord
	OutputStats     DBSRecord
	Status          string
}

// helper function to compare input/output dbs record stats
func compareStats(istats, ostats *DBSRecord) string {
	var out []string
	var test bool
	if istats.NumLumis == ostats.NumLumis {
		test = true
	} else {
		test = false
		out = append(out, fmt.Sprintf("number of lumis differ %d != %d", istats.NumLumis, ostats.NumLumis))
	}
	if istats.NumFiles == ostats.NumFiles {
		test = true
	} else {
		test = false
		out = append(out, fmt.Sprintf("number of files differ %d != %d", istats.NumFiles, ostats.NumFiles))
	}
	if istats.NumEvents == ostats.NumEvents {
		test = true
	} else {
		test = false
		out = append(out, fmt.Sprintf("number of files differ %d != %d", istats.NumEvents, ostats.NumEvents))
	}
	if istats.NumBlocks == ostats.NumBlocks {
		test = true
	} else {
		test = false
		out = append(out, fmt.Sprintf("number of files differ %d != %d", istats.NumBlocks, ostats.NumBlocks))
	}
	if test {
		return "OK"
	}
	return "WARNING: " + strings.Join(out, ", ")
}

// helper function to concurrently check DBS infor for given list of workflows
func concurrentCheck(wflows []string) ([]Record, error) {
	ch := make(chan []Record)
	defer close(ch)
	umap := map[string]bool{}
	for _, w := range wflows {
		umap[w] = true // keep track of processed workflows below
		go func(wflow string, c chan<- []Record) {
			records, err := check(wflow, false)
			if err != nil {
				umap[wflow] = false
				log.Println("fail to process %s, error %v", wflow, err)
			}
			c <- records
		}(w, ch)
	}
	exit := false
	var out []Record
	for {
		select {
		case records := <-ch:
			for _, r := range records {
				out = append(out, r)
				delete(umap, r.Workflow) // remove Url from map
			}
		default:
			if len(umap) == 0 { // no more requests, merge data records
				exit = true
			}
			time.Sleep(time.Duration(10) * time.Millisecond) // wait for response
		}
		if exit {
			break
		}
	}
	return out, nil
}

// helper function to check workflow against DBS
func check(workflow string, verbose bool) ([]Record, error) {
	var out []Record
	rec, err := callReqMgr(workflow, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to get ReqMgr data for %s, %v", workflow, err)
		return out, err
	}

	// extract from JSON TotalInputLumis, InputDataset, and list of OutputDatasets
	input := rec.InputDataset
	if input == "" {
		input = rec.Task1.InputDataset
	}
	durl := fmt.Sprintf("https://cmsweb.cern.ch/dbs/prod/global/DBSReader/filesummaries?dataset=%s", input)
	dbsInputRec, err := dbsCall(durl, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to get DBS data for %s, %v", input, err)
		return out, err
	}
	for _, output := range rec.OutputDatasets {
		durl = fmt.Sprintf("https://cmsweb.cern.ch/dbs/prod/global/DBSReader/filesummaries?dataset=%s", output)
		dbsOutputRec, err := dbsCall(durl, verbose)
		if err != nil {
			fmt.Printf("ERROR: unable to get DBS data for %s, %v", output, err)
			return out, err
		}
		rec := Record{
			Workflow:        workflow,
			TotalInputLumis: rec.TotalInputLumis,
			InputDataset:    input,
			OutputDataset:   output,
			InputStats:      *dbsInputRec,
			OutputStats:     *dbsOutputRec,
			Status:          compareStats(dbsInputRec, dbsOutputRec),
		}
		out = append(out, rec)
	}
	return out, nil
}

// DBSRecord represents filesummaries record we need to parse
type DBSRecord struct {
	NumLumis  int64 `json:"num_lumi"`
	NumFiles  int64 `json:"num_file"`
	NumEvents int64 `json:"num_event"`
	NumBlocks int64 `json:"num_block"`
}

// helper function to perform dbs call
func dbsCall(rurl string, verbose bool) (*DBSRecord, error) {
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
	var records []DBSRecord
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

// Task represents task stucture of ReqMgr2
type Task struct {
	InputDataset string
}

// ReqMgrRecord represents subset of reqmgr record we need to parse
type ReqMgrRecord struct {
	Task1           Task
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
		log.Println("ReqMgr2 data\n", string(data))
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
