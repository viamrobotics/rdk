# Dishcare Matrix Demo

## TODO
* James
    * package viam-server for jetson
* Eric/Fahmina
    * write instructions to download package, and provide them the config file from app.viam.com that we will set up for them in advance
    * give instructions on how to use local UI (probably use screenshots)
    * write a brief overview on how the operation of how python script works so that they can easily adapt to however they'd prefer to ingest the above data

## Packaging
* Add viam.json for jetson to this directory (from https://app.viam.com/api/json1/config?id=6f08f096-d6da-4d3b-9545-b0a81c9aadf6&location=unyyjqwtoa&client=true)
* Run `make package`
* Send them `viam.tgz`

## Dishcare Instructions

### Dependencies
* python3
* python3-pip

### Running 
1. Untar provided .tgz: `tar -xf viam.tgz`
1. Run `make setup`
1. Run `python3 dump.py local.jetson.unyyjqwtoa.viam.cloud 8080`

### UI

* UI is at http://local.jetson.unyyjqwtoa.viam.cloud:8080/

