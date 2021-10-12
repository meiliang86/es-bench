install:
	go build -o es-bench cmd/main.go 
	docker build -t alexshtin/es-bench:$(IMAGE_TAG) .
	docker push alexshtin/es-bench:$(IMAGE_TAG)
	helm install es-bench helm-chart

uninstall:
	helm uninstall es-bench

query:
	go run cmd/main.go --env production query --records 1000 --pf 100 --index temporal_es_bench_v1_test1

manager:
	go run cmd/main.go --env production manager --records 100000 --pf 10000 --index temporal_es_bench_v1_test1
