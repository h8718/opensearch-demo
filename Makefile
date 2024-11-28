OPENSEARCH_VERSION := 2.9.0

.PHONY: all
all: down up

.PHONY: down
down:
	docker compose down --remove-orphans -v

.PHONY: up
up:
	OPENSEARCH_VERSION=$(OPENSEARCH_VERSION) docker-compose up --build
