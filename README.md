# hbsqueue

HostBill Service Queue — orchestrates tenant provisioning and management
workflows across VCD, Zerto, Keycloak, HostBill, Reddit, and Active Directory.

Built on [River](https://riverqueue.com), a Postgres-backed job queue for Go.

## Requirements

- Go 1.26+
- PostgreSQL 18+

## Getting Started

```sh
git clone https://github.com/CloudKey-io/hbsqueue.git
cd hbsqueue
cp .envrc.example .envrc
$EDITOR .envrc
```

## Configuration

All configuration is via environment variables.

| Variable       | Default | Description                   |
| -------------- | ------- | ----------------------------- |
| `PORT`         | `8080`  | HTTP listen port              |
| `ENV`          | `dev`   | Environment (dev / prod)      |
| `API_KEY`      | —       | API key for /api/v1/\* routes |
| `DATABASE_URL` | —       | Postgres connection string    |

## Project Layout

```
cmd/hbsqueue/
internal/
  config/
  httpapi/
```
