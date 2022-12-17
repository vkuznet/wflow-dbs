package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
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
	var lock = sync.RWMutex{}
	ch := make(chan []Record)
	defer close(ch)
	umap := make(map[string]bool)
	for _, w := range wflows {
		lock.Lock()
		umap[w] = true // keep track of processed workflows below
		lock.Unlock()
		go func(wflow string, c chan<- []Record) {
			records, err := check(wflow, false)
			if err != nil {
				lock.Lock()
				umap[wflow] = false
				lock.Unlock()
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
				lock.Lock()
				delete(umap, r.Workflow) // remove Url from map
				lock.Unlock()
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
