#!/bin/bash

./brokerGenConfig.sh
#./mqbroker -n rocketmq-namesrv-service.rocketmq-operator:9876
#./mqbroker -n 192.168.196.49:9876;192.168.122.177:9876
export JAVA_OPT=" -Duser.home=/opt/store"
./mqbroker -n $NAMESRV_ADDRESS -c /opt/rocketmq-4.3.0/conf/broker.conf
