FROM ubuntu:24.04

COPY configmap-controller /usr/local/bin/

RUN chmod +x /usr/local/bin/configmap-controller

CMD ["configmap-controller"]