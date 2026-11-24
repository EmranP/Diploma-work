# Database model

The application uses PostgreSQL through GORM. The following descriptions reflect the JSON-visible fields in the current model definitions. Association-only fields and internal secrets are not exposed in JSON.

## User

| JSON field  | Type      | Notes                                             |
| ----------- | --------- | ------------------------------------------------- |
| `id`        | integer   | Primary key                                       |
| `email`     | string    | Unique, required                                  |
| `role`      | string    | Required and indexed; currently `user` or `admin` |
| `createdAt` | date-time | Creation timestamp                                |

`PasswordHash` is excluded from JSON serialization. The `User` association on a scan is also excluded.

## Scan

| JSON field      | Type          | Notes                                                            |
| --------------- | ------------- | ---------------------------------------------------------------- |
| `id`            | integer       | Primary key                                                      |
| `userId`        | integer       | Owner ID                                                         |
| `url`           | string        | Submitted URL text                                               |
| `normalizedUrl` | string        | Normalized URL; initially empty                                  |
| `status`        | string        | `pending`, `processing`, `completed`, or `failed`                |
| `riskLevel`     | string        | `unknown`, `low`, `medium`, `high`, or `critical`                |
| `score`         | integer       | Analyzer score; current analyzer caps it at 100                  |
| `error`         | string        | Omitted from JSON when empty                                     |
| `createdAt`     | date-time     | Creation timestamp                                               |
| `updatedAt`     | date-time     | Last update timestamp                                            |
| `indicators`    | array or null | Related indicators; may be null in the immediate create response |

`ProcessingToken` is internal and excluded from JSON. The `User` association is excluded from JSON.

## Indicator

| JSON field    | Type    | Notes                              |
| ------------- | ------- | ---------------------------------- |
| `id`          | integer | Primary key                        |
| `scanId`      | integer | Parent scan ID                     |
| `code`        | string  | Indicator code                     |
| `description` | string  | Human-readable description         |
| `weight`      | integer | Contribution to the analyzer score |

The reverse `Scan` association is excluded from JSON.

## Audit log

The `AuditLogModel` stores `id`, `userId`, `action`, `entity`, `entityId` and `createdAt`. It is written when a scan is created. There is no public HTTP endpoint for audit-log retrieval in the current router.

## Ownership and relationships

A scan belongs to one user and has zero or more indicators. The model declares cascading updates and deletes for the relevant relationships. Scan creation and its `scan.created` audit-log record are persisted in one database transaction.

The application applies user ownership constraints in scan queries rather than retrieving all rows and filtering them in application memory.
