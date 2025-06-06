# Code-Grader

**Code-Grader** is a containerized, self-hostable web application designed for the automatic evaluation and grading of programming assignments. Inspired by ReCodEx, it provides a modern interface and a scalable backend to streamline the process of submitting, evaluating, and grading code in educational or training environments.

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
- **Framework**: [Svelte](https://svelte.dev/)
- **Language**: TypeScript
- **Tooling**: Vite
- **Responsibilities**:
  - Role-based dashboards and interfaces
  - Assignment browsing, submission upload, and result display
  - Teacher tools for class and assignment management

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
The backend expects two variables to be set (usually via a `.env` file):

- `DATABASE_URL` – PostgreSQL connection string
- `JWT_SECRET` – secret used to sign authentication tokens

### Services
- **frontend**: Svelte SPA
- **backend**: Gin-based Go API
- **db**: PostgreSQL
- **worker**: Background task processor for grading

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

> **Note**: Data models for entities such as users, classes, assignments, test cases, and submissions are still being finalized. This section will be updated once the schema is defined.

---

## License

This project is in early development. Licensing will be determined prior to public release.

---

## Contact

For development updates or inquiries, please contact the project maintainer.
