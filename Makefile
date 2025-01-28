# Makefile
ENV := $(PWD)/.env

# Environment variables for project
include $(ENV)

postgres:
	docker run --name postgres-container -p 5432:5432 -e POSTGRES_USER=${POSTGRES_USER} -e POSTGRES_PASSWORD=${POSTGRES_PASSWORD} -d postgres:latest
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
migrateup:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose up
migratedown:
	migrate -path db/migrations -database "$(POSTGRES_URL)" -verbose down
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/ChokeGuy/simple-bank/db/sqlc Store
.PHONY: postgres createdb dropdb sqlc migrateup migratedown test server mock