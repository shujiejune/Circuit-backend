## How To Test

1. Start Docker container

```sh
make up
```

2. Apply database migrations

```sh
make migrate-up
```

3. Connect to the database

DBeaver, pgAdmin, psql, whatever you like

4. Seed with test data (Optional)

Generate hashed password with script:

```sh
go run ./misc/hash-password/main.go alice-password
```

Copy and paste the hashed password from terminal to ./internal/migrations/seed.sql

Seed test data

```sh
make db-seed
```

5. Run Go application

```sh
go run cmd/api/main.go
```

## Features Roadmap

Backend
- user
  - authentication
    - email and password
      - sign up
      - activate account (30 min expiry) and first log in
      - log in (30 days expiry)
      - forgot password (send reset password request)
      - reset password (15 min expiry)
      - if fail to receive email, resend
    - OAuth 2.0 (Google)
  - addresses
    - list all addresses
    - add a new address
    - update an address
    - delete an address
  - profile
    - get profile
    - update profile
      - nickname
      - avatar url
