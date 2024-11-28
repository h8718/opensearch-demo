OPENSEARCH_VERSION := 2.9.0
PORT := 8070

.PHONY: all
all: down up

.PHONY: down
down:
	docker compose down --remove-orphans -v

.PHONY: up
up:
	OPENSEARCH_VERSION=$(OPENSEARCH_VERSION) PORT=$(PORT) docker-compose up --build
