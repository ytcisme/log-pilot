log-pilot
=========

[![CircleCI](https://circleci.com/gh/AliyunContainerService/log-pilot.svg?style=svg)](https://circleci.com/gh/AliyunContainerService/log-pilot)

`log-pilot` is an awesome docker log tool. With `log-pilot` you can collect logs from docker hosts and send them to your centralized log system such as elasticsearch, graylog2, awsog and etc. `log-pilot` can collect not only docker stdout but also log file that inside docker containers.

Try it
======

Prerequisites:

- docker-compose >= 1.6
- Docker Engine >= 1.10

```
# download log-pilot project
git clone git@github.com:AliyunContainerService/log-pilot.git
# build log-pilot image
cd log-pilot/ && ./build-image.sh
# quick start
cd quickstart/ && ./run
```

Then access kibana under the tips. You will find that tomcat's has been collected and sended to kibana.

Create index:
![kibana](quickstart/Kibana.png)

Query the logs:
![kibana](quickstart/Kibana2.png)

Quickstart
