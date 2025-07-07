## How To Test

1. First-time setup

If it's the first time setting up the project, or have changed the Dockerfile or go.mod, run:

```sh
make build
```

2. Start all services

Start both the postgreSQL database and Go backend server:

```sh
make up
```

2. Apply database migrations

```sh
make migrate-up
```

3. Seed with test data (Optional)

Generate hashed password with script:

```sh
go run ./misc/hash-password/main.go alice-password
```

Copy and paste the hashed password from terminal to ./internal/migrations/seed.sql

Seed test data

```sh
make db-seed
```

4. Check the logs

```sh
make logs
```

5. Test the API

Now the Go API is listening on http://localhost:8080
the database is accessible on localhost:5432

(optional) connect to the database with DBeaver/pgAdmin/psql/whatever you like
(must do) manually test the endpoints through Postman/curl

6. Stop the application

To stop the running Docker container:

```sh
make stop
```

To stop the container and delete the database volume:

```sh
make down
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
