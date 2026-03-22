.PHONY: dev build migrate-up migrate-down migrate-create sqlc test lint docker-up docker-down promote-admin demote-admin promote-employee demote-employee

-include .env
export

GOBIN   := $(shell $(shell which go || echo /usr/local/go/bin/go) env GOPATH)/bin
MIGRATE := $(GOBIN)/migrate
SQLC    := $(GOBIN)/sqlc

dev:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

migrate-up:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@read -p "Migration name: " name; \
	$(MIGRATE) create -ext sql -dir migrations -seq $$name

sqlc:
	$(SQLC) generate

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down

promote-admin:
	@test -n "$(EMAIL)" || (echo "Usage: make promote-admin EMAIL=you@example.com" && exit 1)
	@echo "Promoting $(EMAIL) to admin..."
	psql "$(DATABASE_URL)" -c " \
		UPDATE users SET role = 'admin', updated_at = now() WHERE email = '$(EMAIL)' AND deleted_at IS NULL; \
		UPDATE \"user\" SET role = 'admin', \"updatedAt\" = now() WHERE email = '$(EMAIL)';"
	@echo "Done. User must log out and back in for the role change to take effect."

demote-admin:
	@test -n "$(EMAIL)" || (echo "Usage: make demote-admin EMAIL=you@example.com" && exit 1)
	@echo "Demoting $(EMAIL) to member..."
	psql "$(DATABASE_URL)" -c " \
		UPDATE users SET role = 'member', updated_at = now() WHERE email = '$(EMAIL)' AND deleted_at IS NULL; \
		UPDATE \"user\" SET role = 'member', \"updatedAt\" = now() WHERE email = '$(EMAIL)';"
	@echo "Done. Demoted $(EMAIL) to member."

promote-employee:
	@test -n "$(EMAIL)" || (echo "Usage: make promote-employee EMAIL=you@example.com" && exit 1)
	@echo "Promoting $(EMAIL) to employee..."
	psql "$(DATABASE_URL)" -c " \
		UPDATE users SET role = 'employee', updated_at = now() WHERE email = '$(EMAIL)' AND deleted_at IS NULL; \
		UPDATE \"user\" SET role = 'employee', \"updatedAt\" = now() WHERE email = '$(EMAIL)';"
	@echo "Done. User must log out and back in for the role change to take effect."

demote-employee:
	@test -n "$(EMAIL)" || (echo "Usage: make demote-employee EMAIL=you@example.com" && exit 1)
	@echo "Demoting $(EMAIL) to member..."
	psql "$(DATABASE_URL)" -c " \
		UPDATE users SET role = 'member', updated_at = now() WHERE email = '$(EMAIL)' AND deleted_at IS NULL; \
		UPDATE \"user\" SET role = 'member', \"updatedAt\" = now() WHERE email = '$(EMAIL)';"
	@echo "Done. Demoted $(EMAIL) to member."
