.PHONY: swag-init migrate-db testifylint testifylint-fix
# This Makefile is used to manage various tasks related to the loci AI project.
swag-init:
	swag init -g ./main.go -o ./docs

migrate-db:
	migrate -database "postgresql://postgres:postgres@localhost:5454/loci-dev?sslmode=disable" -path ./app/db/migrations up

testifylint:
	testifylint ./...

testifylint-fix:
	testifylint -fix ./...

