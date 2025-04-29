swag-init:
	swag init -g ./main.go -o ./docs

migrate-db:
	migrate -database "postgresql://postgres:postgres@localhost:5454/wanderwise-ai-dev?sslmode=disable" -path ./app/db/migrations up

