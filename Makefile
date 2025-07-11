# Load environment variables from .env file for use in this Makefile
# This makes sure DATABASE_URL is available for the migrate commands.
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: help up down stop logs migrate-up migrate-down db-seed

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  up             - Start all services in detached mode"
	@echo "  down           - Stop and remove all services and volumes"
	@echo "  stop           - Stop all running services"
	@echo "  logs           - View logs for all services"
	@echo "  migrate-up     - Apply all new database migrations"
	@echo "  migrate-down   - Roll back the last database migration"
	@echo "  db-seed        - Seed the database with initial test data"

up:
	@echo "Starting Docker containers..."
	docker compose up -d

down:
	@echo "Stopping and removing Docker containers and volumes..."
	docker compose down -v

stop:
	@echo "Stopping Docker containers..."
	docker compose stop

logs:
	@echo "Tailing logs..."
	docker compose logs -f

migrate-up:
	@echo "Applying database migrations..."
	migrate -database "${DATABASE_URL}" -path internal/migrations up

migrate-down:
	@echo "Rolling back last database migration..."
	migrate -database "${DATABASE_URL}" -path internal/migrations down

db-seed:
	@echo "Seeding database with test data..."
	psql "${DATABASE_URL}" -f internal/migrations/seed.sql
