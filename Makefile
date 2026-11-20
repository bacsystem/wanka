# Entorno local de Wanka. `make help` lista los targets. Guía: docs/local-environment.md
SHELL := /bin/sh
.DEFAULT_GOAL := help

ENV_FILE := .env.local
STATE := .local
-include $(ENV_FILE)
export

WANKA_PG_PORT  ?= 55432
WANKA_API_PORT ?= 58080
WANKA_WEB_PORT ?= 3400
DC := docker compose --env-file $(ENV_FILE) -f compose.yaml
PG_HOST := 127.0.0.1:$(WANKA_PG_PORT)

# Variables que lee la API (derivadas de .env.local)
API_ENV = HTTP_ADDR=127.0.0.1:$(WANKA_API_PORT) \
  DATABASE_URL_APP=postgres://wanka_app:$(WANKA_ROLE_PASSWORD)@$(PG_HOST)/wanka \
  DATABASE_URL_PLATFORM=postgres://wanka_platform:$(WANKA_ROLE_PASSWORD)@$(PG_HOST)/wanka \
  DATABASE_URL_WORKER=postgres://wanka_worker:$(WANKA_ROLE_PASSWORD)@$(PG_HOST)/wanka \
  DATABASE_URL_WEBHOOKS=postgres://wanka_webhooks:$(WANKA_ROLE_PASSWORD)@$(PG_HOST)/wanka
SEED_URL = postgres://postgres:$(POSTGRES_SUPERUSER_PASSWORD)@$(PG_HOST)/wanka
MIGRATOR_URL = postgres://wanka_migrator:$(WANKA_ROLE_PASSWORD)@$(PG_HOST)/wanka

.PHONY: help env up down reset roles migrate seed api web test test-api rls-check lint logs prune-volumes _up _start-api _start-web _stop-procs

help: ## Lista los targets
	@awk 'BEGIN{FS=":.*## "} /^[a-z-]+:.*## /{printf "  %-10s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

env: ## Genera .env.local con claves aleatorias si falta
	@if [ ! -f $(ENV_FILE) ]; then \
	  cp .env.example $(ENV_FILE); \
	  while grep -q CHANGE_ME $(ENV_FILE); do \
	    v=$$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 32); \
	    sed "1,/CHANGE_ME/s/CHANGE_ME/$$v/" $(ENV_FILE) > $(ENV_FILE).tmp && mv $(ENV_FILE).tmp $(ENV_FILE); \
	  done; \
	  chmod 600 $(ENV_FILE); echo "creado $(ENV_FILE)"; \
	fi

up: env ## PostgreSQL + roles + migraciones + API + web (en segundo plano)
	@$(MAKE) --no-print-directory _up

# Se invoca de nuevo para que make relea .env.local si `env` lo acaba de crear
_up:
	@$(DC) up -d --wait postgres
	@$(MAKE) --no-print-directory roles migrate
	@mkdir -p $(STATE)
	@$(MAKE) --no-print-directory _start-api _start-web
	@echo ""
	@echo "Wanka local arriba:"
	@echo "  web        http://localhost:$(WANKA_WEB_PORT)"
	@echo "  api        http://127.0.0.1:$(WANKA_API_PORT)  (GET /healthz, /readyz)"
	@echo "  postgres   127.0.0.1:$(WANKA_PG_PORT) (usuario postgres, base wanka; claves en $(ENV_FILE))"
	@echo "  logs: make logs   |   apagar: make down   |   borrar todo: make reset"

_start-api:
	@if [ -f $(STATE)/api.pid ] && kill -0 $$(cat $(STATE)/api.pid) 2>/dev/null; then echo "api ya corre"; else \
	  (cd apps/api && go build -o ../../$(STATE)/wanka ./cmd/wanka) && \
	  ( $(API_ENV) nohup $(STATE)/wanka serve --plane=tenant >$(STATE)/api.log 2>&1 & echo $$! >$(STATE)/api.pid ); \
	  for i in $$(seq 1 40); do curl -fsS http://127.0.0.1:$(WANKA_API_PORT)/readyz >/dev/null 2>&1 && break; sleep 0.5; done; \
	  curl -fsS http://127.0.0.1:$(WANKA_API_PORT)/readyz >/dev/null && echo "api lista" || { echo "la api no respondio; mira $(STATE)/api.log"; exit 1; }; fi

_start-web:
	@if [ ! -f $(STATE)/web.pid ] && lsof -nP -iTCP:$(WANKA_WEB_PORT) -sTCP:LISTEN >/dev/null 2>&1; then echo "el puerto $(WANKA_WEB_PORT) ya esta en uso; cambia WANKA_WEB_PORT en $(ENV_FILE)" >&2; exit 1; fi
	@if [ -f $(STATE)/web.pid ] && kill -0 $$(cat $(STATE)/web.pid) 2>/dev/null; then echo "web ya corre"; else \
	  ( WANKA_API_URL=http://127.0.0.1:$(WANKA_API_PORT) nohup pnpm --filter web exec next dev -p $(WANKA_WEB_PORT) >$(STATE)/web.log 2>&1 & echo $$! >$(STATE)/web.pid ); \
	  for i in $$(seq 1 120); do curl -fsS -o /dev/null http://localhost:$(WANKA_WEB_PORT)/ 2>/dev/null && break; sleep 1; done; \
	  curl -sS -o /dev/null http://localhost:$(WANKA_WEB_PORT)/ && echo "web lista" || { echo "la web no respondio; mira $(STATE)/web.log"; exit 1; }; fi

_stop-procs:
	@for p in web api; do \
	  if [ -f $(STATE)/$$p.pid ]; then pid=$$(cat $(STATE)/$$p.pid); \
	    pkill -P $$pid 2>/dev/null; kill $$pid 2>/dev/null; rm -f $(STATE)/$$p.pid; fi; done

down: _stop-procs ## Detiene API, web y PostgreSQL (conserva los datos)
	@if [ -f $(ENV_FILE) ]; then $(DC) down; fi

reset: env _stop-procs ## Borra contenedor, volumen de datos y estado local (pide confirmar; CONFIRM=1 lo salta)
	@if [ "$$CONFIRM" != "1" ]; then printf "Borra TODOS los datos locales de wanka-local. Seguir? [s/N] "; read a; [ "$$a" = "s" ] || { echo cancelado; exit 1; }; fi
	@if [ -f $(ENV_FILE) ]; then $(DC) --profile tools down -v --remove-orphans; fi
	rm -f .local/api.log .local/web.log .local/wanka

roles: env ## Bootstrap de roles y claves locales (idempotente)
	@$(DC) up -d --wait postgres
	@$(DC) --profile tools run --rm roles

migrate: env ## Aplica las migraciones (wanka migrate up)
	@cd apps/api && DATABASE_URL_MIGRATOR=$(MIGRATOR_URL) go run ./cmd/wanka migrate up

seed: env ## Empresa de prueba y un usuario por rol (idempotente; solo local)
	@cd apps/api && WANKA_ENV=local DATABASE_URL_SEED=$(SEED_URL) go run ./cmd/wanka dev-seed

api: env ## API en primer plano (go run), para depurar
	@cd apps/api && $(API_ENV) go run ./cmd/wanka serve --plane=tenant

web: env ## Web en primer plano (pnpm dev)
	WANKA_API_URL=http://127.0.0.1:$(WANKA_API_PORT) pnpm --filter web exec next dev -p $(WANKA_WEB_PORT)

test: test-api ## Pruebas de la API y de la web
	pnpm test

test-api: env ## Pruebas de la API contra el PostgreSQL local
	@cd apps/api && WANKA_TEST_ADMIN_URL=postgres://postgres:$(POSTGRES_SUPERUSER_PASSWORD)@$(PG_HOST)/wanka WANKA_TEST_ROLE_PASSWORD=$(WANKA_ROLE_PASSWORD) go test -race ./...

rls-check: ## Comprobacion de RLS (Docker, contenedor propio)
	sh docs/architecture/data/rls-check/run.sh

lint: ## eslint (web), gofmt y go vet (api)
	pnpm lint
	cd apps/api && test -z "$$(gofmt -l .)" && go vet ./...

prune-volumes: ## Lista volumenes Docker anonimos colgados (no borra nada)
	@echo "Volumenes anonimos colgados (sin contenedor). Pueden ser de otros proyectos: revisa antes de borrar."
	@for v in $$(docker volume ls -qf dangling=true | grep -E '^[0-9a-f]{64}$$'); do \
	  echo "$$(docker volume inspect -f '{{.CreatedAt}}' $$v)  $$v"; done
	@echo "Para borrar uno: docker volume rm <nombre>"

logs: ## Sigue los logs de PostgreSQL, API y web
	@mkdir -p $(STATE); touch $(STATE)/api.log $(STATE)/web.log
	@$(DC) logs -f --tail=20 postgres & tail -n 20 -f $(STATE)/api.log $(STATE)/web.log
