# CodeEdu

**CodeEdu** is a containerized, self-hostable web application designed for the automatic evaluation and grading of programming assignments. Inspired by ReCodEx, it provides a modern interface and a scalable backend to streamline the process of submitting, evaluating, and grading code in educational or training environments.

---

## Project Goals

- Provide role-based access for students, teachers, and administrators
- Automate code grading using isolated Docker containers
- Deliver detailed test case feedback to students
- Support easy deployment and scalability using Docker
- Start with Python language support and allow for future language expansion

---

## Technology Stack

### Frontend
- **Framework**: [SvelteKit](https://kit.svelte.dev/)
- **Language**: TypeScript
- **Tooling**: Vite
- **Responsibilities**:
  - Role-based dashboards and interfaces
  - Assignment browsing, submission upload, and result display
  - Teacher tools for class and assignment management
  - Upload Python `unittest` files to generate test cases

### Backend
- **Language**: Go
- **Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin)
- **Responsibilities**:
  - REST API for frontend communication
  - User authentication and role management
  - Assignment and test case handling
  - Submission evaluation and grading
  - Result persistence and aggregation

### Database
- **System**: PostgreSQL
- **Responsibilities**:
  - Persistent storage of users, assignments, submissions, and results

---

## Code Execution Environment

Submitted code is executed in secure Docker containers with the following constraints:

- Read-only file system
- No network access
- Limited CPU and memory
- Execution timeouts
- Non-root user

The backend initiates and manages container execution for each submission and test case.

---

## Background Task Processing

The system uses a custom task queue implemented in Go (using goroutines and channels) to offload grading tasks from the main application server. This ensures:

- Responsiveness of the main API
- Parallel processing of submissions
- Graceful recovery from failed tasks

---

## Grading Policies

Grading behavior is defined per assignment and supports:

- **All-or-Nothing**: Full marks only if all test cases pass
- **Percentage-Based**: Partial credit based on test case success rate
- **Custom Weighting**: (planned) Assign custom scores to individual test cases

---

## Security Features

- **Password Hashing**: bcrypt
- **Authentication**: JWT-based tokens with expiration and roles
- **Container Execution**: Isolated environment with tight resource control
- **File Upload**: Strict validation of file types and execution inputs

---

## Deployment

The system is fully Dockerized for simple setup using `docker-compose`.

### Environment Variables
The backend expects several variables to be set (usually via a `.env` file):

- `DATABASE_URL` – PostgreSQL connection string
- `JWT_SECRET` – secret used to sign authentication tokens
- `ADMIN_EMAIL` – email for the seeded administrator account
- `ADMIN_PASSWORD` – password for the seeded administrator account
- `BAKALARI_BASE_URL` – base URL of the Bakaláři API v3 instance

When this variable is configured, the frontend login page presents a
"Bakalari" tab that communicates with Bakaláři directly so credentials are
never sent to the CodeEdu server.

You can copy `backend/.env.example` and adjust it for your environment.

### Services
- **frontend**: SvelteKit static build
- **backend**: Gin-based Go API
- **db**: PostgreSQL
- **worker**: Background task processor for grading
- Ensure the Docker image `python:3.11` is available locally. If it's missing,
  pull it once with `docker pull python:3.11`.
- After modifying the frontend, rebuild it with `npm run build` so the Go
  server can serve the updated static files.

---

## Development Roadmap

### Phase 1: MVP
- Basic user registration and login
- Class and assignment creation by teachers
- Python submissions with automated grading
- Result reporting and submission status

### Phase 2: UI and Feature Enhancements
- Improved UX/UI
- Time and memory limits per test case
- Submission history and feedback view

### Phase 3: Language Support and Administration
- Support for C++, Java, and other languages
- Administrator tools and system monitoring
- Optional plagiarism detection

---

## Data Model Design

The current PostgreSQL schema is stored in `backend/schema.sql` and is executed automatically when the backend starts. This eliminates the need for external migration tools during early development.

Each assignment stores a `max_points` value and a `grading_policy`. The policy controls how points are awarded and can be either `all_or_nothing` or `weighted`.

---

## API Notes

The backend now exposes two additional endpoints:

- `DELETE /api/users/:id` – Admin only. Deletes the specified user and cascades removal of related classes, assignments and submissions. Returns `204` on success.
- `GET /api/my-submissions` – Student endpoint returning the authenticated user's submissions ordered by creation date.
- `PUT /api/assignments/:id/publish` – Teacher/admin endpoint to publish an assignment once it's ready.
- `POST /login-bakalari` – Authenticate via Bakaláři API v3 using username and password.
  The endpoint stores the user's Bakaláři class abbreviation and short ID when available.
- `POST /api/bakalari/atoms` – Teacher endpoint returning the teacher's class atoms from Bakaláři.
- `POST /api/classes/:id/import-bakalari` – Import all students from a selected Bakaláři class into the local class.
- `POST /api/assignments/:id/tests/upload` – Teacher/admin endpoint that parses a Python `unittest` file and creates individual test cases for each method.

---

## License

This project is in early development. Licensing will be determined prior to public release.

---

## Contact

For development updates or inquiries, please contact the project maintainer.
