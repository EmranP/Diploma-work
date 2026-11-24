# HTTP API

## Base path

The API exposes the health endpoint at `/health`. Application endpoints are grouped under `/api`.

## Authentication

All endpoints under `/api` require a JWT Bearer access token except `POST /api/auth/login`. The health endpoint is public.

Send the token in the request header:

```http
Authorization: Bearer <access-token>
```

See [Authentication](authentication.md) for token claims and roles.

## Endpoints

| Method | Path              | Access        | Purpose                                  |
| ------ | ----------------- | ------------- | ---------------------------------------- |
| `GET`  | `/health`         | Public        | Health check                             |
| `POST` | `/api/auth/login` | Public        | Authenticate and receive an access token |
| `POST` | `/api/scans`      | Authenticated | Create and queue a scan                  |
| `GET`  | `/api/scans`      | Authenticated | List visible scans                       |
| `GET`  | `/api/scans/{id}` | Authenticated | Retrieve a scan and indicators           |
| `GET`  | `/api/stats`      | Admin         | Retrieve aggregate scan statistics       |

The machine-readable contract is [OpenAPI](../api/openapi.yaml).

## Health

`GET /health` returns HTTP `200`:

```json
{
	"status": "ok"
}
```

## Login

`POST /api/auth/login`

Request:

```json
{
	"email": "user@example.com",
	"password": "example-password"
}
```

The server trims surrounding whitespace from the email and converts it to lowercase. Both fields must be non-empty.

Successful response (`200`):

```json
{
	"accessToken": "<signed-jwt>",
	"tokenType": "Bearer",
	"expiresIn": 28800,
	"user": {
		"id": 12,
		"email": "user@example.com",
		"role": "user"
	}
}
```

`expiresIn` is measured in seconds. The current access-token lifetime is 8 hours.

Possible errors:

- `400` — malformed request body or missing email/password.
- `401` — email or password is incorrect.
- `500` — unexpected server or account-configuration error.

## Create a scan

`POST /api/scans`

Request:

```json
{
	"url": "https://example.com/login"
}
```

A non-empty URL string is required. The owner ID is taken from the authenticated token; clients must not send a `userId` to select an owner.

Successful response (`202 Accepted`) is the newly persisted scan. It normally has `pending` status and `unknown` risk level. The scan identifier is published to the background queue; the final analysis is not returned synchronously.

Possible errors:

- `400` — malformed body or empty URL.
- `401` — missing or invalid authentication.
- `500` — database or other internal error.
- `503` — scan service or message queue unavailable, or task publication failed.

## List scans

`GET /api/scans?limit=20`

The optional `limit` parameter has a default of `20` and an accepted range of `1` through `100`. An invalid or out-of-range value is replaced by `20` by the current implementation; it does not produce a validation error.

Response (`200`):

```json
{
	"items": [],
	"total": 0,
	"limit": 20
}
```

Results are sorted by `createdAt` descending. A regular user receives only their own scans. An administrator receives scans across all users.

## Get a scan

`GET /api/scans/{id}`

The `id` path parameter must be a positive integer. A successful response (`200`) contains the scan and its `indicators` collection.

Possible errors:

- `400` — invalid scan ID.
- `401` — missing or invalid authentication.
- `404` — scan does not exist or is outside the authenticated user's visibility scope.
- `500` — unexpected database or server error.

The API intentionally returns `404` for a scan owned by another regular user rather than disclosing its existence with `403`.

## Statistics

`GET /api/stats` is restricted to the `admin` role.

Response (`200`):

```json
{
	"totalScans": 100,
	"highRiskScans": 15
}
```

`highRiskScans` counts scans whose risk level is `high` or `critical`.

Possible errors:

- `401` — missing or invalid authentication.
- `403` — authenticated user is not an administrator.
- `500` — unexpected database or server error.

## Error response format

The Fiber error handler serializes errors using this shape:

```json
{
	"error": "Human-readable message"
}
```

Clients should rely on the HTTP status and the presence of the `error` field, not on exact message wording.

## Scan state and result fields

The scan status is one of:

- `pending`
- `processing`
- `completed`
- `failed`

The risk level is one of:

- `unknown` — initial value before analysis completes.
- `low`
- `medium`
- `high`
- `critical`

The `normalizedUrl` is initially an empty string and is populated by background analysis. The `error` property is omitted from JSON when empty. The `indicators` property can be `null` in the immediate create response before analysis has produced indicators.
