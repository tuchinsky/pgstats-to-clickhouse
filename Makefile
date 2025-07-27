project-name=pgstats

docker-compose-file=./docker/docker-compose.yaml

up:
	docker-compose -p ${project-name} -f ${docker-compose-file} up -d --build

	sleep 3

	docker-compose -p ${project-name} -f ${docker-compose-file} exec -T postgres psql -U postgres  < ./docker/fixtures/postgres.sql
	docker-compose -p ${project-name} -f ${docker-compose-file} exec -T clickhouse clickhouse-client -mn < ./docker/fixtures/clickhouse.sql

down:
	docker-compose -p ${project-name} -f ${docker-compose-file} down --volumes

fmt:
	@go fmt ./...

vet:
	@go vet ./...

lint:
	@golangci-lint run

build:
	mkdir -p ./bin
	@go build -o ./bin/pgstats-to-clickhouse ./cmd/pgstats-to-clickhouse

.PHONY: up down fmt vet lint build
