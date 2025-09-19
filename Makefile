SHELL := /bin/bash
APP_NAME := go-exchange
IMAGE := $(APP_NAME):local
BUILD_DIR := ./build

.PHONY: help deps test build image compose-up compose-down run clean fmt vet

help:
	@echo "help: mostra esta ajuda"
	@echo "deps: baixa dependências (go mod download)"
	@echo "test: roda os testes (go test ./...)"
	@echo "build: compila binário local em $(BUILD_DIR)"
	@echo "image: constrói imagem docker local ($(IMAGE))"
	@echo "compose-up: levanta redis + app via docker-compose"
	@echo "compose-down: encerra stack docker-compose"
	@echo "run: roda binário local com env do .env"
	@echo "clean: remove artefatos de build"

deps:
	@echo "==> baixando dependências"
	go mod download

test:
	@echo "==> rodando testes"
	go test ./...

build: deps
	@echo "==> compilando binário"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME) ./

image: build
	@echo "==> construindo imagem docker"
	docker build -t $(IMAGE) .

compose-up:
	@echo "==> docker-compose up -d"
	docker-compose up -d --build

compose-down:
	@echo "==> docker-compose down"
	docker-compose down

run: build
	@echo "==> rodando binário local"
	# carregar variáveis do .env se existir
	set -a; [ -f .env ] && source .env; set +a; \
	$(BUILD_DIR)/$(APP_NAME)

clean:
	@echo "==> limpando"
	rm -rf $(BUILD_DIR)/$(APP_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...
