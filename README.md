# Beam

A fast, simple file transfer service. Upload files from the CLI, share a link, download from anywhere.

## Architecture

```
beam/
├── cmd/
│   ├── beam/              # CLI client
│   │   └── main.go
│   └── server/            # Server backend
│       └── main.go
├── internal/
│   ├── core/              # Shared types (filetree, compression, payload)
│   └── server/
│       ├── api/           # HTTP handlers, middleware, routing
│       ├── config/        # Environment-based configuration
│       ├── database/      # PostgreSQL connection, models, repository
│       ├── service/       # Business logic
│       └── storage/       # File storage, cleanup service
├── migrations/            # SQL migration files
└── .env.example           # Configuration template
```

## Prerequisites

- Go 1.21+
- PostgreSQL 14+

## Setup

### 1. Database

```bash
createdb beam
createuser beam -P   # set password: beam
```

### 2. Configuration

```bash
cp .env.example .env
# Edit .env with your database credentials and preferences
```

### 3. Run the Server

```bash
# Source environment variables
export $(cat .env | xargs)

# Run the server
go run ./cmd/server
```

The server will automatically run database migrations on startup.

### 4. Use the CLI

```bash
# Upload a file
go run ./cmd/beam myfile.txt

# Upload a directory
go run ./cmd/beam ./my-project/
```

## API Endpoints

| Method   | Path                       | Description                       |
|----------|----------------------------|-----------------------------------|
| `POST`   | `/api/upload`              | Upload a ZIP file                 |
| `GET`    | `/d/:id`                   | Download a file                   |
| `GET`    | `/api/info/:id`            | Get upload metadata               |
| `DELETE` | `/api/delete/:id/:token`   | Delete upload with deletion token |
| `GET`    | `/health`                  | Health check                      |
| `GET`    | `/api/stats`               | Server statistics                 |

### Upload

```bash
curl -X POST http://localhost:8080/api/upload \
  -F "file=@archive.zip" \
  -F "password=optional-secret"
```

Response:

```json
{
  "id": "aBcDeFgHiJkLmNoP",
  "download_url": "http://localhost:8080/d/aBcDeFgHiJkLmNoP",
  "deletion_token": "del_xYzAbCdEfGhIjKlMnOpQrStUv",
  "expires_at": "2026-02-14T20:00:00Z",
  "filename": "archive.zip",
  "size": 1048576
}
```

### Download

```bash
# Without password
curl -OJ http://localhost:8080/d/aBcDeFgHiJkLmNoP

# With password
curl -OJ "http://localhost:8080/d/aBcDeFgHiJkLmNoP?password=secret"
```

### Delete

```bash
curl -X DELETE http://localhost:8080/api/delete/aBcDeFgHiJkLmNoP/del_xYzAbCdEfGhIjKlMnOpQrStUv
```

## Configuration Reference

| Variable              | Default                                                    | Description                  |
|-----------------------|------------------------------------------------------------|------------------------------|
| `PORT`                | `8080`                                                     | Server listen port           |
| `BASE_URL`            | `http://localhost:8080`                                    | Public URL for download links|
| `DATABASE_URL`        | `postgres://beam:beam@localhost:5432/beam?sslmode=disable` | PostgreSQL connection string |
| `STORAGE_PATH`        | `./storage/files`                                          | File storage directory       |
| `MAX_FILE_SIZE`       | `5368709120` (5 GB)                                        | Max upload size in bytes     |
| `DEFAULT_EXPIRY_HOURS`| `168` (7 days)                                             | File expiration time         |
| `CLEANUP_INTERVAL_HOURS`| `1`                                                      | Expired file cleanup interval|
| `RATE_LIMIT_RPS`      | `10`                                                       | Upload rate limit per IP     |
| `RATE_LIMIT_BURST`    | `20`                                                       | Upload rate limit burst      |
