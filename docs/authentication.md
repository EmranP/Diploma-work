# Authentication and authorization

## Login flow

Clients authenticate with `POST /api/auth/login`, providing an email address and password. The server trims the email, converts it to lowercase, looks up the account, and verifies the password hash with bcrypt.

On success, the server returns a signed JWT access token and a public user object. Password hashes are never serialized in the user model.

## Access token

| Property                         | Current value                       |
| -------------------------------- | ----------------------------------- |
| Token type                       | Bearer                              |
| Signing algorithm                | HS256                               |
| Issuer (`iss`)                   | `phishing-detector`                 |
| Audience (`aud`)                 | `phishing-detector-api`             |
| Subject (`sub`)                  | User ID encoded as a decimal string |
| Role claim                       | `role`                              |
| Lifetime                         | 8 hours (`28800` seconds)           |
| Required expiration              | Yes                                 |
| Issued-at validation             | Enabled                             |
| Minimum configured secret length | 32 bytes                            |

The token also contains `iat`, `nbf`, and `exp` registered claims. The middleware restricts accepted signing methods to HS256 and validates issuer, audience, expiration, issued-at, subject and role.

## Roles

The application defines two roles:

- `user`
- `admin`

The authorization middleware normalizes role values to lowercase before checking them.

## Access matrix

| Endpoint               | Public |   user    |   admin   |
| ---------------------- | :----: | :-------: | :-------: |
| `GET /health`          |  Yes   |    Yes    |    Yes    |
| `POST /api/auth/login` |  Yes   |    Yes    |    Yes    |
| `POST /api/scans`      |   No   |    Yes    |    Yes    |
| `GET /api/scans`       |   No   | Own scans | All scans |
| `GET /api/scans/{id}`  |   No   | Own scans | All scans |
| `GET /api/stats`       |   No   |    No     |    Yes    |

A missing, malformed, invalid or expired token results in `401 Unauthorized`. A valid token without the required admin role results in `403 Forbidden` for the statistics endpoint.

## Ownership enforcement

The scan query scope is applied at the database query level. Regular users' queries are constrained by their authenticated user ID. Administrators do not receive this ownership filter.

When a regular user requests a scan belonging to a different user, the query returns no visible record and the API responds with `404 Not Found`.

## Secret handling

The JWT secret must be configured outside source code and contain at least 32 bytes. Never expose it in API responses, logs, committed environment files or frontend source code. The frontend should store and send the access token according to the application's client-side security design; the current API returns it in the login JSON response and does not set an authentication cookie.
