# Hospital Backend Development Plan

## Project Structure
```
hospital-backend/
├── main.go
├── go.mod
├── Dockerfile
├── docker-compose.yml
├── nginx.conf
├── README.md
├── models/
│   ├── hospital.go
│   ├── staff.go
│   └── patient.go
├── handlers/
│   ├── staff_handler.go
│   └── patient_handler.go
├── middleware/
│   └── auth.go
├── database/
│   └── db.go
└── tests/
    ├── staff_test.go
    └── patient_test.go
```

## API Specification

### POST /staff/create
- Request Body:
  ```json
  {
    "username": "string",
    "password": "string",
    "hospital": "string"
  }
  ```
- Response: 201 Created or error

### POST /staff/login
- Request Body:
  ```json
  {
    "username": "string",
    "password": "string",
    "hospital": "string"
  }
  ```
- Response:
  ```json
  {
    "token": "jwt_token"
  }
  ```

### GET /patient/search
- Headers: Authorization: Bearer <token>
- Query Params (optional): citizen_id, passport, first_name, last_name, phone, email, hn
- Response: Array of patient objects

## ER Diagram

```
+----------------+       +----------------+
|   Hospitals    |       |     Staffs     |
+----------------+       +----------------+
| id (PK)        |<------| id (PK)        |
| name (unique)  |       | username (unique)|
+----------------+       | password_hash  |
                         | hospital_id (FK)|
                         +----------------+

+----------------+       +----------------+
|   Hospitals    |       |    Patients    |
+----------------+       +----------------+
| id (PK)        |<------| id (PK)        |
| name (unique)  |       | first_name     |
+----------------+       | last_name      |
                         | first_name_th  |
                         | last_name_th   |
                         | birth_date     |
                         | hn (unique)    |
                         | citizen_id     |
                         | passport       |
                         | phone          |
                         | email          |
                         | gender         |
                         | hospital_id (FK)|
                         +----------------+
```

## Business Logic
- Staff can only search patients from their own hospital
- Authentication required for patient search
- Hospitals are created automatically if not exist during staff creation

## Testing Plan
- Unit tests for all APIs
- Positive and negative test cases
- Database integration tests