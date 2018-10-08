FROM alpine
MAINTAINER yarntime@163.com

ADD build/bin/rocketmq-operator /usr/local/bin/rocketmq-operator

ENTRYPOINT ["/usr/local/bin/rocketmq-operator"]
