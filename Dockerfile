FROM debian:stable-slim
RUN apt update
RUN apt -y install ca-certificates

WORKDIR /etc/es-bench
COPY ./es-bench /usr/local/bin/
COPY ./config /etc/es-bench/config

#CMD ["es-bench", "--env", "production", "manager", "--records", "1000000", "--pf", "1000", "--index", "temporal_es_bench_v1_test1"]

