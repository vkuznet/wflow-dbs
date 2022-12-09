# wflow-dbs
[![Go CI build](https://github.com/vkuznet/wflow-dbs/actions/workflows/go.yml/badge.svg)](https://github.com/vkuznet/wflow-dbs/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vkuznet/wflow-dbs)](https://goreportcard.com/report/github.com/vkuznet/wflow-dbs)
CMS workflow DBS checker obtains DBS statistics for given workflows.

The provided executable can be run as CLI or web server.

```
# web interface:
./wflow-dbs -webConfig server.json

# curl call via GET:
curl http://localhost:888/stats?workflow=pdmvserv_task_EXO-RunIISummer20UL18NanoAODv9-00948__v1_T_220227_022251_1319
[
   {
      "Workflow": "pdmvserv_task_EXO-RunIISummer20UL18NanoAODv9-00948__v1_T_220227_022251_1319",
      "TotalInputLumis": 401,
      "InputDataset": "/TTbar01Jets_TypeIHeavyN-Mu_LepSMTop_3L_LO_MN20_TuneCP5_13TeV-madgraphMLM-pythia8/RunIISummer20UL18MiniAODv2-106X_upgrade2018_realistic_v16_L1v1-v3/MINIAODSIM",
      "OutputDataset": "/TTbar01Jets_TypeIHeavyN-Mu_LepSMTop_3L_LO_MN20_TuneCP5_13TeV-madgraphMLM-pythia8/RunIISummer20UL18NanoAODv9-106X_upgrade2018_realistic_v16_L1v1-v1/NANOAODSIM",
      "InputStats": {
         "num_lumi": 401,
         "num_file": 23,
         "num_event": 397395,
         "num_block": 16,
         "num_invalid_files": 0
      },
      "OutputStats": {
         "num_lumi": 6,
         "num_file": 1,
         "num_event": 6001,
         "num_block": 1,
         "num_invalid_files": 0
      },
      "Status": "WARNING: number of lumis differ 401 != 6, number of files differ 397395 != 6001"
   }
]

# curl call via POST request
curl -X POST -H "Content-type: application/json" -d@/tmp/w.json http://cmsweb-test9.cern.ch:30123/stats
...


# CLI interface:
./wflow-dbs -workflow pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352
[
   {
      "Workflow": "pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352",
      "TotalInputLumis": 31372,
      "InputDataset": "/LowEGJet/Run2017G-v1/RAW",
      "OutputDataset": "/LowEGJet/Run2017G-09Aug2019_UL2017-v2/AOD",
      "InputStats": {
         "num_lumi": 31372,
         "num_file": 23666,
         "num_event": 967230225,
         "num_block": 52,
         "num_invalid_files": 0
      },
      "OutputStats": {
         "num_lumi": 31372,
         "num_file": 12327,
         "num_event": 967230225,
         "num_block": 36,
         "num_invalid_files": 29
      },
      "Status": "OK"
   },
   {
      "Workflow": "pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352",
      "TotalInputLumis": 31372,
      "InputDataset": "/LowEGJet/Run2017G-v1/RAW",
      "OutputDataset": "/LowEGJet/Run2017G-09Aug2019_UL2017-v2/MINIAOD",
      "InputStats": {
         "num_lumi": 31372,
         "num_file": 23666,
         "num_event": 967230225,
         "num_block": 52,
         "num_invalid_files": 0
      },
      "OutputStats": {
         "num_lumi": 31372,
         "num_file": 1904,
         "num_event": 967230225,
         "num_block": 23,
         "num_invalid_files": 1
      },
      "Status": "OK"
   }
```
