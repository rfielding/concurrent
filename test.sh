#!/bin/bash

rm data.txt
#go run concurrent.go > data.txt
go run cmd/stats/stats.go | tail +5 > data.txt
./usl.py
