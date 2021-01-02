#!/bin/bash

go run concurrent.go > data.txt
./usl.py
