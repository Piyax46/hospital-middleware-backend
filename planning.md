# Hospital Middleware Backend — Development Plan

---

## 1. Project Structure

```
hospital-middleware-backend/
├── main.go                    # Application entry point, route registration
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
├── Dockerfile                 # Multi-stage Docker build
├── docker-compose.yml         # Orchestrates app + postgres + nginx
├── nginx.conf                 # Reverse proxy config (port 80 → 8080)
├── planning.md                # This document
├── README.md                  # Quick-start guide
│
├── his/
│   └── client.go              # Hospital A HIS API client (interface + impl)
│
├── models/
│   ├── hospital.go            # Hospital model
│   ├── staff.go               # Staff model
│   └── patient.go             # Patient model (mirrors HIS response fields)
│
├── handlers/
│   ├── staff_handler.go       # POST /staff/create, POST /staff/login
│   └── patient_handler.go     # GET /patient/search (with HIS sync)
│
├── middleware/
│   └── auth.go                # JWT auth middleware (AuthRequired)
│
├── database/
│   └── db.go                  # Postgres connection, GORM AutoMigrate, test DB
│
└── tests/
    ├── setup_test.go          # TestMain — connects SQLite in-memory test DB
    ├── staff_test.go          # Unit tests for /staff/create and /staff/login
    └── patient_test.go        # Unit tests for /patient/search
```

---

## 2. API Specification

### Base URL
- **Development (Docker):** `http://localhost` (via Nginx on port 80)
- **Direct:**              `http://localhost:8080`

### Authentication
Protected endpoints require a JWT Bearer token in the `Authorization` header:
```
Authorization: Bearer <token>
```
Tokens are issued at login, valid for **24 hours**, signed with `HS256`.

---

### POST /staff/create

Create a new staff member. If the hospital does not exist, it is created automatically.

**Request Body (JSON)**

| Field      | Type   | Required | Description                  |
|------------|--------|----------|------------------------------|
| `username` | string | ✅       | Must be unique within hospital |
| `password` | string | ✅       | Minimum 6 characters         |
| `hospital` | string | ✅       | Hospital name                |

**Example Request**
```json
{
  "username": "nurse_a",
  "password": "password123",
  "hospital": "Bangkok Hospital"
}
```

**Response: 201 Created**
```json
{
  "message": "Staff created successfully",
  "data": {
    "id": 1,
    "username": "nurse_a",
    "hospital_id": 1,
    "hospital_name": "Bangkok Hospital"
  }
}
```

**Error Responses**

| Status | Condition                                  |
|--------|--------------------------------------------|
| `400`  | Missing or invalid fields (e.g. password < 6 chars) |
| `409`  | Username already exists in the same hospital |
| `500`  | Internal server error                      |

---

### POST /staff/login

Authenticate staff and receive a JWT token.

**Request Body (JSON)**

| Field      | Type   | Required | Description   |
|------------|--------|----------|---------------|
| `username` | string | ✅       |               |
| `password` | string | ✅       |               |
| `hospital` | string | ✅       | Hospital name |

**Example Request**
```json
{
  "username": "nurse_a",
  "password": "password123",
  "hospital": "Bangkok Hospital"
}
```

**Response: 200 OK**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "staff": {
    "id": 1,
    "username": "nurse_a",
    "hospital_id": 1,
    "hospital_name": "Bangkok Hospital"
  }
}
```

**Error Responses**

| Status | Condition                              |
|--------|----------------------------------------|
| `400`  | Missing required fields                |
| `401`  | Wrong username, password, or hospital  |

---

### GET /patient/search

Search for patients. Results are **always scoped to the authenticated staff member's hospital**. All query parameters are optional; omitting all returns every patient in the hospital.

**Headers**
```
Authorization: Bearer <jwt_token>
```

**Query Parameters (all optional)**

| Parameter     | Type   | Match Type              | Description                     |
|---------------|--------|-------------------------|---------------------------------|
| `national_id` | string | Exact                   | Thai national ID                |
| `passport_id` | string | Exact                   | Passport number                 |
| `first_name`  | string | Partial, case-insensitive | Searches both EN and TH fields |
| `middle_name` | string | Partial, case-insensitive | Searches both EN and TH fields |
| `last_name`   | string | Partial, case-insensitive | Searches both EN and TH fields |
| `date_of_birth` | string | Exact date `YYYY-MM-DD` |                                |
| `phone_number`| string | Exact                   |                                 |
| `email`       | string | Partial, case-insensitive |                                |
| `patient_hn`  | string | Exact                   | Hospital Number                 |

**HIS Sync Behaviour**
When `national_id` or `passport_id` is provided, the middleware first calls the Hospital A HIS API (`GET https://hospital-a.api.co.th/patient/search/{id}`) and upserts the result into the local database before querying. HIS errors are non-fatal — local results are always returned.

**Example Request**
```
GET /patient/search?national_id=1234567890123
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**
```json
{
  "total": 1,
  "data": [
    {
      "id": 1,
      "first_name_th": "สมชาย",
      "middle_name_th": "",
      "last_name_th": "ใจดี",
      "first_name_en": "Somchai",
      "middle_name_en": "",
      "last_name_en": "Jaidee",
      "date_of_birth": "1990-05-15T00:00:00Z",
      "patient_hn": "HN-001",
      "national_id": "1234567890123",
      "passport_id": "",
      "phone_number": "0812345678",
      "email": "somchai@example.com",
      "gender": "M",
      "hospital_id": 1,
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Error Responses**

| Status | Condition                                              |
|--------|--------------------------------------------------------|
| `400`  | Invalid `date_of_birth` format (expected `YYYY-MM-DD`) |
| `401`  | Missing, invalid, or expired JWT token                 |
| `500`  | Database error                                         |

---

## 3. ER Diagram

```
┌──────────────────────┐
│       Hospital        │
├──────────────────────┤
│ id          PK       │
│ name        UNIQUE   │
│ created_at           │
│ updated_at           │
└──────────┬───────────┘
           │ 1
           │
    ┌──────┴──────┐
    │             │
    │ *           │ *
┌───┴──────────────────┐    ┌──────────────────────────┐
│        Staff          │    │         Patient            │
├──────────────────────┤    ├──────────────────────────┤
│ id          PK       │    │ id            PK          │
│ username             │    │ first_name_th             │
│ password_hash        │    │ middle_name_th            │
│ hospital_id  FK ─────┘    │ last_name_th              │
│ created_at           │    │ first_name_en             │
│ updated_at           │    │ middle_name_en            │
└──────────────────────┘    │ last_name_en              │
                            │ date_of_birth             │
                            │ patient_hn    INDEX       │
                            │ national_id   INDEX       │
                            │ passport_id   INDEX       │
                            │ phone_number              │
                            │ email                     │
                            │ gender        (M/F)       │
                            │ hospital_id   FK ─────────┘
                            │ created_at                │
                            │ updated_at                │
                            └───────────────────────────┘

Composite Unique Indexes:
  staffs   → (username, hospital_id)
  patients → (patient_hn, hospital_id)
```

---

## 4. Database Schema

### Hospital
| Column       | Type        | Constraints        |
|--------------|-------------|--------------------|
| `id`         | uint        | PK, Auto-increment |
| `name`       | varchar     | UNIQUE, NOT NULL   |
| `created_at` | timestamp   |                    |
| `updated_at` | timestamp   |                    |

### Staff
| Column          | Type      | Constraints                          |
|-----------------|-----------|--------------------------------------|
| `id`            | uint      | PK, Auto-increment                   |
| `username`      | varchar   | NOT NULL                             |
| `password_hash` | varchar   | NOT NULL (bcrypt)                    |
| `hospital_id`   | uint      | FK → Hospital, NOT NULL              |
| `created_at`    | timestamp |                                      |
| `updated_at`    | timestamp |                                      |

Composite unique: `(username, hospital_id)` — same username is allowed across different hospitals.

### Patient
| Column           | Type      | Constraints                      |
|------------------|-----------|----------------------------------|
| `id`             | uint      | PK, Auto-increment               |
| `first_name_th`  | varchar   |                                  |
| `middle_name_th` | varchar   |                                  |
| `last_name_th`   | varchar   |                                  |
| `first_name_en`  | varchar   | NOT NULL                         |
| `middle_name_en` | varchar   |                                  |
| `last_name_en`   | varchar   | NOT NULL                         |
| `date_of_birth`  | timestamp |                                  |
| `patient_hn`     | varchar   | NOT NULL, INDEX                  |
| `national_id`    | varchar   | INDEX                            |
| `passport_id`    | varchar   | INDEX                            |
| `phone_number`   | varchar   |                                  |
| `email`          | varchar   |                                  |
| `gender`         | varchar   | "M" or "F"                       |
| `hospital_id`    | uint      | FK → Hospital, NOT NULL          |
| `created_at`     | timestamp |                                  |
| `updated_at`     | timestamp |                                  |

Composite unique: `(patient_hn, hospital_id)`.

---

## 5. Business Logic

- Staff can **only search patients from their own hospital** (enforced via JWT `hospital_id` claim)
- Hospitals are **created automatically** if they don't exist during staff creation
- When searching by `national_id` or `passport_id`, the middleware **syncs with HIS** before returning local results
- HIS errors are **non-fatal** — local database results are always returned
- Passwords are hashed with **bcrypt** (cost factor 10)

---

## 6. Tech Stack

| Component  | Technology              |
|------------|-------------------------|
| Language   | Go 1.23                 |
| Framework  | Gin                     |
| ORM        | GORM                    |
| Database   | PostgreSQL 15           |
| Auth       | JWT (HS256, 24h expiry) |
| Container  | Docker + Docker Compose |
| Proxy      | Nginx (port 80)         |
| Test DB    | SQLite (in-memory)      |
| Test lib   | testify                 |

---

## 7. Running the Project

### Start all services
```bash
docker-compose up --build
```
The API is available at `http://localhost` (Nginx) or `http://localhost:8080` (direct).

### Run unit tests
```bash
go test ./tests/... -v
```

### Environment variables (docker-compose sets these automatically)
| Variable      | Default                            |
|---------------|------------------------------------|
| `DB_HOST`     | `db`                               |
| `DB_USER`     | `user`                             |
| `DB_PASSWORD` | `password`                         |
| `DB_NAME`     | `hospital`                         |
| `DB_PORT`     | `5432`                             |
| `JWT_SECRET`  | `change-this-in-production`        |
