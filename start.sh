#!/bin/sh

nohup memcached -u nobody &
nohup ./server &
sleep 2
./ot-client