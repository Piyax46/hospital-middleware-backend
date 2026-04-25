# Hospital Backend

A Go backend for hospital management system using Gin, PostgreSQL, Docker, and Nginx.

## Features

- Staff creation and authentication
- Patient search with hospital-based access control
- JWT-based authentication
- Dockerized with Nginx reverse proxy

## API Endpoints

### Staff
- `POST /staff/create` - Create a new staff member
- `POST /staff/login` - Login and get JWT token

### Patient
- `GET /patient/search` - Search patients (requires authentication)

## Running the Application

1. Start the services:
   ```bash
   docker-compose up --build
   ```

2. The API will be available at `http://localhost`

## Database Schema

### Hospitals
- id: Primary Key
- name: Unique hospital name

### Staffs
- id: Primary Key
- username: Unique username
- password_hash: Hashed password
- hospital_id: Foreign Key to Hospitals

### Patients
- id: Primary Key
- first_name, last_name: English names
- first_name_th, last_name_th: Thai names
- birth_date: Date of birth
- hn: Hospital Number (unique)
- citizen_id: Thai Citizen ID
- passport: Passport number
- phone: Phone number
- email: Email address
- gender: Gender
- hospital_id: Foreign Key to Hospitals

## ER Diagram

```
Hospitals (1) -- (Many) Staffs
Hospitals (1) -- (Many) Patients
```

## Unit Tests

Run tests with:
```bash
go test ./tests/...
```

## Project Structure

- `main.go` - Application entry point
- `models/` - Database models
- `handlers/` - API handlers
- `middleware/` - Authentication middleware
- `database/` - Database connection
- `tests/` - Unit tests
- `Dockerfile` - Docker build file
- `docker-compose.yml` - Docker services
- `nginx.conf` - Nginx configuration