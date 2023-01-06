package main

import (
	"context"
	"encoding/json"
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
	rec.TotalFileLumis = totalLumis
	rec.UniqueFileLumis = uniqueLumis

	totalLumis, err = dbsFilesummariesLumis(blocks, verbose)
	if err != nil {
		fmt.Printf("ERROR: unable to call dbsFilesummariesLumis for %s, %v", dataset, err)
		return rec, err
	}
	rec.FilesummariesLumis = totalLumis
	return rec, nil
}

// DBSRecord represents filesummaries record we need to parse
type DBSRecord struct {
	NumLumis           int64 `json:"num_lumi"`            // output of filesummaries?dataset=xxx
	NumFiles           int64 `json:"num_file"`            // output of filesummaries?dataset=xxx
	NumEvents          int64 `json:"num_event"`           // output of filesummaries?dataset=xxx
	NumBlocks          int64 `json:"num_block"`           // output of filesummaries?dataset=xxx
	TotalFileLumis     int64 `json:"num_file_lumis"`      // output of filelumis?block_name=xxx
	UniqueFileLumis    int64 `json:"unique_file_lumis"`   // output of filelumis?block_name=xxx
	FilesummariesLumis int64 `json:"filesummaries_lumis"` // output of filesummaries?block_name=xxx
	NumInvalidFiles    int64 `json:"num_invalid_files"`   // number of invalid files
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
	if len(arr) != 2 {
		log.Println("### unable to extract block ID from", blk)
		return blk
	}
	return arr[1]
}

// SyncMap represents map to keep track of go routines
// type SyncMap map[string]bool
type SyncMap struct {
	sync.Map
}

// Len implements map size function
func (m *SyncMap) Len() int {
	count := 0
	m.Range(func(k, v any) bool {
		count++
		return true
	})
	return count
}

// Lumi represents part of filesummaries data structure
type Lumi struct {
	NumLumi int64 `json:"num_lumi"`
}

// DbsListEntry identifies types used by list's generics function
type DbsListEntry interface {
	RunLumi | Lumi
}

func dbsApiCall[T DbsListEntry](rurl, bid string, verbose bool, out *[]T) {
	time0 := time.Now()
	defer func() {
		if verbose {
			log.Printf("finished %s in %s\n", bid, time.Since(time0))
		}
	}()
	if verbose {
		log.Println("dbs call", rurl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*60))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", rurl, nil)
	if err != nil {
		log.Println("ERROR: dbsApiCall new request", err)
		return
	}
	req.Header.Add("Accept", "application/ndjson")
	client := HttpClient(verbose)
	resp, err := client.Do(req)
	atomic.AddUint64(&TotalURLCalls, 1)
	if err != nil {
		if verbose {
			log.Println("ERROR: dbsApiCall client.Do", err)
		}
		return
	}
	defer resp.Body.Close()

	// we'll use json decoder to walk through our json stream (ndjson)
	// see explanation about json decoder in this blog post:
	// https://mottaquikarim.github.io/dev/posts/you-might-not-be-using-json.decoder-correctly-in-golang/
	dec := json.NewDecoder(resp.Body)
	for {
		var rec T
		err := dec.Decode(&rec)
		if err == io.EOF {
			return
		}
		*out = append(*out, rec)
	}
}

// helper function to get unique number of lumis for given list of blocks
func dbsBlocksLumis(blocks []string, verbose bool) (int64, int64, error) {
	time0 := time.Now()
	var out []RunLumi
	group := pool.Group()
	for _, b := range blocks {
		bid := blockID(b)
		rurl := fmt.Sprintf("%s/filelumis?block_name=%s", dbsUrl, url.QueryEscape(b))

		// usage of pool provides controlled (fixed size) environment to call DBS
		// where at most we will place number of calls limited by max pool size
		group.Submit(func() {
			dbsApiCall(rurl, bid, verbose, &out)
		})
	}
	group.Wait()

	if verbose {
		log.Printf("Make %d calls to DBS to fetch block lumis in %s\n", len(blocks), time.Since(time0))
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

// helper function to get unique number of lumis for given list of blocks
func dbsFilesummariesLumis(blocks []string, verbose bool) (int64, error) {
	time0 := time.Now()
	var out []Lumi
	group := pool.Group()
	for _, b := range blocks {
		bid := blockID(b)
		rurl := fmt.Sprintf("%s/filesummaries?block_name=%s", dbsUrl, url.QueryEscape(b))

		// usage of pool provides controlled (fixed size) environment to call DBS
		// where at most we will place number of calls limited by max pool size
		group.Submit(func() {
			dbsApiCall(rurl, bid, verbose, &out)
		})
	}
	group.Wait()

	if verbose {
		log.Printf("Make %d calls to DBS to fetch block lumis in %s\n", len(blocks), time.Since(time0))
	}
	var totalLumis int64
	for _, r := range out {
		totalLumis += r.NumLumi
	}
	return totalLumis, nil
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
