export PROJECT_ROOT=$(shell pwd)

# App-Pro ===========================================================
up:
	@docker compose --env-file backend/.env -f docker-compose.yml up --build

up-d:
	@docker compose --env-file backend/.env -f docker-compose.yml up -d --build

up-nc:
	@docker compose --env-file backend/.env -f docker-compose.yml build --no-cache
	@docker compose --env-file backend/.env -f docker-compose.yml up -d --force-recreate

recreate:
	@docker compose --env-file backend/.env -f docker-compose.yml up -d --no-deps --force-recreate $(service)

stop:
	@docker compose --env-file backend/.env -f docker-compose.yml down $(services)

stops:
	@docker compose --env-file backend/.env -f docker-compose.yml down

clear:
	@read -p "Clear all volume files of the environment? Data loss is dangerous. [y/N]: " ans; \
	if [ "$$ans" = "y" ] || [ "$$ans" = "yes" ]; then \
		for vol in diploma_pgdata diploma_cache diploma_pgadmin diploma_natsdata diploma_docs; do \
			if docker volume inspect "$$vol" >/dev/null 2>&1; then \
				docker volume rm "$$vol"; \
				echo "Volume '$$vol' removed successfully."; \
			else \
				echo "Volume '$$vol' not found, skipped."; \
			fi; \
		done; \
		echo "Environment volumes cleanup completed."; \
	else \
		echo "Operation cancelled."; \
	fi