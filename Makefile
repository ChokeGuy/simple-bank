# Makefile
ENV := $(PWD)/.env

# Environment variables for project
include $(ENV)

postgres:
	docker run --name postgres-container --network bank-network -p 5432:5432 -e POSTGRES_USER=${POSTGRES_USER} -e POSTGRES_PASSWORD=${POSTGRES_PASSWORD} -d postgres:latest
createdb:
	docker exec -it postgres-container createdb --username=root --owner=root simple-bank
dropdb:
	docker exec -it postgres-container dropdb --force simple-bank
sqlc:
	sqlc generate
test:
	go test -v -cover ./...
server:
	go run cmd/main.go
migratecreate:
	migrate create -ext sql -dir db/migrations -seq $(name)
migrateup:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose up
migrateup1:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose up 1
migratedown:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose down
migratedown1:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose down 1
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/ChokeGuy/simple-bank/db/sqlc Store

.PHONY: postgres createdb dropdb sqlc migrateup migratedown migrateup1 migratedown1 test server mock migratecreate