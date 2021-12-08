FROM debian:stable-slim
RUN apt update
RUN apt -y install ca-certificates
RUN apt -y install curl
RUN apt -y install vim

WORKDIR /etc/es-bench
COPY ./es-bench /usr/local/bin/
COPY ./config /etc/es-bench/config
COPY ./schema /etc/es-bench/schema

#CMD ["es-bench", "--env", "production", "manager", "--records", "1000000", "--pf", "1000", "--index", "temporal_es_bench_v1_test1"]

