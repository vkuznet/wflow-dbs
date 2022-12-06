# wflow-dbs
[![Go CI build](https://github.com/vkuznet/wflow-dbs/actions/workflows/go-ci.yml/badge.svg)](https://github.com/vkuznet/wflow-dbs/actions/workflows/go-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vkuznet/wflow-dbs)](https://goreportcard.com/report/github.com/vkuznet/wflow-dbs)
CMS workflow DBS checker checks given workflow stats against DBS server and
return input/output dataset along with corresponding lumi info.
```
# build
make

# usage:
./wflow-dbs -workflow pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352
[
   {
      "Workflow": "pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352",
      "TotalInputLumis": 31372,
      "InputDataset": "/LowEGJet/Run2017G-v1/RAW",
      "OutputDataset": "/LowEGJet/Run2017G-09Aug2019_UL2017-v2/AOD",
      "InputDatasetLumis": 31372,
      "OutputDatasetLumis": 31372
   },
   {
      "Workflow": "pdmvserv_Run2017G_LowEGJet_09Aug2019_UL2017_220531_180507_3352",
      "TotalInputLumis": 31372,
      "InputDataset": "/LowEGJet/Run2017G-v1/RAW",
      "OutputDataset": "/LowEGJet/Run2017G-09Aug2019_UL2017-v2/MINIAOD",
      "InputDatasetLumis": 31372,
      "OutputDatasetLumis": 31372
   }
]
```
