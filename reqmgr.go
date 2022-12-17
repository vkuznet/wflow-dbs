package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

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
