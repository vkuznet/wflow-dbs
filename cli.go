package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const dbsUrl string = "https://cmsweb.cern.ch/dbs/prod/global/DBSReader"

// TotalURLCalls counts total number of URL calls we made
var TotalURLCalls uint64

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
	if istats.NumLumis != ostats.NumLumis {
		out = append(out, fmt.Sprintf("number of lumis differ %d != %d", istats.NumLumis, ostats.NumLumis))
	}
	if istats.NumEvents != ostats.NumEvents {
		out = append(out, fmt.Sprintf("number of events differ %d != %d", istats.NumEvents, ostats.NumEvents))
	}
	msg := "OK"
	if len(out) != 0 {
		msg = "WARNING: " + strings.Join(out, ", ")
	}
	//     if istats.NumInvalidFiles != ostats.NumInvalidFiles {
	//         msg = fmt.Sprintf("%s, but some files were invalidated", msg)
	//     }
	return msg
}

// helper function to concurrently check DBS infor for given list of workflows
func concurrentCheck(wflows []string) ([]Record, error) {
	ch := make(chan []Record)
	defer close(ch)
	umap := make(map[string]bool)
	for _, w := range wflows {
		umap[w] = true // keep track of processed workflows below
		go func(wflow string, c chan<- []Record) {
			records, err := check(wflow, false)
			if err != nil {
				umap[wflow] = false
				log.Printf("fail to process %s, error %v", wflow, err)
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

// helper function to get DBS stats for total/valid number of files
func dbsStats(dataset string, verbose bool) (*DBSRecord, error) {
	rec, err := dbsCall(dataset, 1, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to call dbsCall for %s, %v", dataset, err)
		return rec, err
	}
	blocks, err := dbsBlocks(dataset, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to call dbsBlocks for %s, %v", dataset, err)
		return rec, err
	}
	totalLumis, uniqueLumis, err := dbsBlocksLumis(blocks, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to call dbsBlocksLumis for %s, %v", dataset, err)
		return rec, err
	}
	rec.TotalBlockLumis = totalLumis
	rec.UniqueBlockLumis = uniqueLumis
	return rec, nil
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
	dbsInputRec, err := dbsStats(input, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to get DBS data for %s, %v", input, err)
		return out, err
	}
	for _, output := range rec.OutputDatasets {
		dbsOutputRec, err := dbsStats(output, verbose)
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
	NumLumis         int64 `json:"num_lumi"`
	NumFiles         int64 `json:"num_file"`
	NumEvents        int64 `json:"num_event"`
	NumBlocks        int64 `json:"num_block"`
	TotalBlockLumis  int64 `json:"num_block_lumis"`
	UniqueBlockLumis int64 `json:"unique_block_lumis"`
	NumInvalidFiles  int64 `json:"num_invalid_files"`
}

// DBSBlocks represents blocks record we need to parse
type DBSBlock struct {
	BlockName string `json:"block_name"`
}

// helper function to get list of blocks for a given dataset
func dbsBlocks(dataset string, verbose bool) ([]string, error) {
	var blocks []string
	rurl := fmt.Sprintf("%s/blocks?dataset=%s", dbsUrl, dataset)
	if verbose {
		log.Println("dbs call", rurl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rurl, nil)
	if err != nil {
		return blocks, err
	}
	req.Header.Add("Accept", "application/json")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	atomic.AddUint64(&TotalURLCalls, 1)
	if err != nil {
		if verbose {
			log.Println("dbsCall client.Do", err)
		}
		return blocks, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	var records []DBSBlock
	err = json.Unmarshal(data, &records)
	if err != nil {
		if verbose {
			log.Println("dbsCall json.Unmarshal", err)
		}
		return nil, err
	}
	for _, rec := range records {
		if !InList(rec.BlockName, blocks) {
			blocks = append(blocks, rec.BlockName)
		}
	}
	return blocks, nil
}

// RunLumi represents run-lumi object
type RunLumi struct {
	Run  int `json:"run_num"`
	Lumi int `json:"lumi_section_num"`
}

// helper function to extract block ID from block name
func blockID(blk string) string {
	arr := strings.Split(blk, "#")
	return arr[1]
}

// GoMap represents map to keep track of go routines
type GoMap map[string]bool

// helper function to yield RunLumi records for given URL with block name
func runLumis(rurl, bid string, verbose bool, ch chan<- RunLumi, umap *GoMap) {
	var lock = sync.Mutex{}
	defer func() {
		lock.Lock()
		delete(*umap, bid) // we done with this stream of data
		if verbose {
			log.Println("delete", bid, "from url map")
		}
		lock.Unlock()
	}()
	if verbose {
		log.Println("dbs call", rurl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*180))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rurl, nil)
	if err != nil {
		log.Println("ERROR: runLumis new request", err)
		return
	}
	req.Header.Add("Accept", "application/ndjson")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	atomic.AddUint64(&TotalURLCalls, 1)
	if err != nil {
		if verbose {
			log.Println("ERROR: runLumis client.Do", err)
		}
		return
	}
	defer resp.Body.Close()

	// we'll use json decoder to walk through our json stream (ndjson)
	// see explanation about json decoder in this blog post:
	// https://mottaquikarim.github.io/dev/posts/you-might-not-be-using-json.decoder-correctly-in-golang/
	dec := json.NewDecoder(resp.Body)
	for {
		var rec RunLumi
		err := dec.Decode(&rec)
		if err == io.EOF {
			return
		}
		ch <- rec
	}
}

// helper function to get unique number of lumis for given list of blocks
func dbsBlocksLumis(blocks []string, verbose bool) (int64, int64, error) {
	ch := make(chan RunLumi)
	defer close(ch)
	umap := make(GoMap)
	for _, b := range blocks {
		bid := blockID(b)
		umap[bid] = true // keep track of processed block ids
		rurl := fmt.Sprintf("%s/filelumis?block_name=%s", dbsUrl, url.QueryEscape(b))
		go runLumis(rurl, bid, verbose, ch, &umap)
	}
	if verbose {
		log.Printf("Make %d calls to DBS to fetch block lumis\n", len(umap))
	}
	exit := false
	var out []RunLumi
	for {
		select {
		case r := <-ch:
			out = append(out, r)
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
	totalLumis := int64(len(out))
	uniqueLumis := int64(len(uniqueRunLumis(out)))
	return totalLumis, uniqueLumis, nil
}

// helper function to get unique number of RunLumi records
func uniqueRunLumis(records []RunLumi) []RunLumi {
	var out []RunLumi
	for _, rec := range records {
		found := false
		for _, r := range out {
			if r.Run == rec.Run && r.Lumi == rec.Lumi {
				found = true
			}
		}
		if !found {
			out = append(out, rec)
		}
	}
	return out
}

// helper function to perform dbs call
func dbsCall(input string, validFileOnly int, verbose bool) (*DBSRecord, error) {
	rurl := fmt.Sprintf("%s/filesummaries?dataset=%s", dbsUrl, input)
	if validFileOnly == 1 {
		rurl = fmt.Sprintf("%s/filesummaries?dataset=%s&validFileOnly=%d", dbsUrl, input, validFileOnly)
	}
	if verbose {
		log.Println("dbs call", rurl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rurl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	atomic.AddUint64(&TotalURLCalls, 1)
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
	rec.NumInvalidFiles = rec.NumFiles - int64(len(records))
	return &rec, nil

}

// WorkflowRecord represent reqmgr map record
type WorkflowRecord map[string]ReqMgrRecord

// ResultRecord represents reqmgr result record
type ResultRecord struct {
	Result []WorkflowRecord
}

// Task represents task structure of ReqMgr2
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rurl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/json")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	atomic.AddUint64(&TotalURLCalls, 1)
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
