#!/bin/bash

go run cmd/stats/stats.go > data2.txt
alpha=`cat data2.txt | head -8 | grep alpha: | awk '{print $2;}'`
beta=`cat data2.txt | head -8 | grep beta: | awk '{print $2;}'`
gamma=`cat data2.txt | head -8 | grep gamma: | awk '{print $2;}'`
cat data2.txt | tail +8 | grep -v NaN > data.txt
rm data2.txt 
./usl.py ${alpha} ${beta} ${gamma}
