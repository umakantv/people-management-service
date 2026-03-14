# People Service

A Go module for managing people and groups they are part of.

## Module

```bash
go mod init github.com/umakantv/people
```

## Dependencies

This project uses [github.com/umakantv/go-utils](https://github.com/umakantv/go-utils) v0.0.2 for reusable HTTP server components:

- **httpserver**: Standardized HTTP server with routing, authentication, logging, and context injection (uses gorilla/mux)
- **logger**: Structured logging via Uber Zap with configurable output
- **errs**: Standardized error handling with predefined HTTP status code mappings
- **db**: Database connection (sqlx) and migration management

## Setup

Install dependencies:

```bash
go mod tidy
```

### Configuration

Copy the example environment file and edit as needed:

```bash
cp .env.example .env
```

The `.env` file supports:

```
DRIVER=sqlite3      # sqlite3, mysql, or postgres
DB=people.db        # File path for SQLite, db name for others
# HOST=localhost    # For MySQL/PostgreSQL
# PORT=3306
# USER=root
# PASSWORD=
```

### Database Migrations

Migrations are stored in `./migrations/` and run automatically on startup. Migration files must follow the format: `<UTC timestamp>_<name>.sql` (14-digit timestamp).

Create a new migration:

```bash
go run github.com/umakantv/go-utils/db/migrations/create-migration.go --name add_groups_table --dir ./migrations
```

Or run migrations manually via CLI:

```bash
go run github.com/umakantv/go-utils/db/migrations/migrate.go --dir ./migrations
```

### Build & Run

Build:

```bash
go build -o people .
```

Run the server:

```bash
./people
```

Or run directly:

```bash
go run .
```

The server starts on port **8080**.

## API Endpoints

All endpoints require no authentication (`AuthType: none`).

### Health Check

```
GET /health
```

Returns the service health status:

```json
{"status": "healthy"}
```

### People API

#### Create Person

```
POST /people
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com",
  "joined_date": "2024-01-15"
}
```

Response (201 Created):

```json
{
  "id": 1,
  "name": "John Doe",
  "email": "john@example.com",
  "is_active": 1,
  "joined_date": "2024-01-15",
  "activated_at": "2024-01-15 10:00:00"
}
```

#### Search People

```
GET /people?q=john
```

Searches by substring match on name or email (case-insensitive). Returns array of people.

Omit `q` to list all people.

#### Update Person

```
PUT /people/{id}
Content-Type: application/json

{
  "name": "John Smith",
  "email": "john.smith@example.com"
}
```

Partial updates supported - only include fields to change. Returns updated person (200 OK).

#### Deactivate Person

```
POST /people/{id}/deactivate
```

Sets `is_active=0` and `deactived_at=now()`. Returns updated person (200 OK).

#### Reactivate Person

```
POST /people/{id}/reactivate
```

Sets `is_active=1` and `activated_at=now()`. Returns updated person (200 OK).

## Project Structure

```
.
├── .env.example    # Example environment configuration
├── config/
│   └── config.go   # Configuration loader (reads .env)
├── go.mod          # Go module definition
├── go.sum          # Dependency checksums
├── handlers/       # HTTP request handlers
│   └── person.go   # Person API handlers
├── main.go         # Application entry point
├── migrations/     # Database migration files
│   └── *.sql
├── models/         # Data models
│   └── person.go   # Person entity and request types
├── repository/     # Database access layer
│   └── person_repo.go  # Person CRUD operations
└── README.md       # This file
```

## Future Modules

This service will expand to include:

- **People**: CRUD operations for individuals ✓ (current)
- **Groups**: CRUD operations for groups
- **Membership**: Manage people-to-group relationships
