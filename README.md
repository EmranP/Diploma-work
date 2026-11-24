# Phishing Detector

Backend monolith for detecting suspicious URLs for the "Türkmentelekom" EAK diploma project.

## Files

- [`api/openapi.yaml`](docs/openapi.yaml) — OpenAPI 3.0.3 specification.
- [`docs/api.md`](docs/api.md) — HTTP endpoints, requests, responses and errors.
- [`docs/authentication.md`](docs/authentication.md) — JWT authentication, roles and ownership rules.
- [`docs/database.md`](docs/database.md) — JSON-visible database model fields and relationships.

## Run

```bash
cd phishing-detector

cp backend/.env.example backend/.env

docker compose up --build || make up || make up-d
```

The API is available at `http://localhost:8080`.

Default administrator credentials are read from `backend/.env`:

```text
admin@turkmentelekom.tm
ChangeMe123!
```

## Components

- Go + Fiber REST API
- PostgreSQL: users, scans, indicators, audit logs
- Redis: 10-minute analysis cache
- NATS: asynchronous scan queue inside the monolith
- Docker Compose: local infrastructure and API startup

The React + TypeScript dashboard is available at `http://localhost:5173` after startup.

## Swagger UI

Swagger UI has not yet been added to the project's Docker Compose configuration. To integrate it safely, merge a Swagger UI service into the existing Compose file and mount this package's `api/openapi.yaml` as `/openapi/openapi.yaml`. The service can then load the specification from `/openapi/openapi.yaml`.

The exact service name, host port and Compose changes should be chosen after reviewing the repository's current Docker Compose file to avoid port conflicts or replacing existing service settings.

## Contract notes

The specification documents the router and handlers supplied for review. The API uses JWT Bearer authentication for `/api` routes except login. Scan analysis is asynchronous and its final result is retrieved through the scan endpoints.
