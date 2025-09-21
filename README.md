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

- Read-only file system (with a small writable tmpfs at `/tmp`)
- No network access by default; egress can be enabled only on dedicated Docker networks or via host-level firewall rules
- Limited CPU and memory
- Execution timeouts
- Non-root user with dropped Linux capabilities and `--security-opt=no-new-privileges`
- Supports rootless Docker where available

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

### Running Containers
1. Start the persistent PostgreSQL database stack once with `docker compose -f docker-compose.db.yml up -d`.
2. Launch or rebuild the application services with `docker compose up -d`.

Stopping the application (`docker compose down`) keeps the database running because it lives in its own Compose project. When you really need to shut the database down, run `docker compose -f docker-compose.db.yml down`.

### Environment Variables
The backend expects several variables to be set (usually via a `.env` file):

- `DATABASE_URL` – PostgreSQL connection string
- `JWT_SECRET` – secret used to sign authentication tokens
- `TURNSTILE_SECRET_KEY` – Cloudflare Turnstile secret used to validate registration challenges
- `ADMIN_EMAIL` – email for the seeded administrator account
- `ADMIN_PASSWORD` – password for the seeded administrator account
- `BAKALARI_BASE_URL` – base URL of the Bakaláři API v3 instance
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD` – SMTP credentials used to send transactional email (port defaults to 587 if omitted)
- `SMTP_FROM` – verified sender address used as the envelope `From`
- `SMTP_FROM_NAME` – optional display name for the `From` header
- `PASSWORD_RESET_BASE_URL` – absolute frontend origin used to render password reset links (for example `https://codedu.example.com`)
- `APP_BASE_URL` – optional origin used in email notifications; defaults to `PASSWORD_RESET_BASE_URL` when unset
- `SMTP_DKIM_SELECTOR`, `SMTP_DKIM_DOMAIN` – optional DKIM selector and signing domain used to align outbound mail
- `SMTP_DKIM_PRIVATE_KEY` or `SMTP_DKIM_PRIVATE_KEY_FILE` – supply the private key PEM used for DKIM signing (inline value or path to a readable file)

When this variable is configured, the frontend login page presents a
"Bakalari" tab that communicates with Bakaláři directly so credentials are
never sent to the CodeEdu server.

The SvelteKit frontend also needs `PUBLIC_TURNSTILE_SITE_KEY` so it can render the
Turnstile widget on the registration page.

You can copy `backend/.env.example` and adjust it for your environment.

### Email Deliverability (SPF, DKIM, DMARC)

- The backend issues messages with the required headers (`From`, `To`, `Date`, `Message-ID`, `MIME-Version`, `Content-Type`, and `Content-Transfer-Encoding`) and derives `Message-ID` values from your domain so they align with DMARC.
- Configure SPF for the domain used in `SMTP_FROM` by publishing a TXT record such as `v=spf1 include:mail.your-provider.example -all` that authorises your SMTP host/IPs.
- Enable DKIM by providing the selector, domain, and private key via the variables above; publish the matching DNS TXT record at `<selector>._domainkey.<domain>` that contains `v=DKIM1; k=rsa; p=<public-key>` (or the Ed25519 equivalent).
- Publish a DMARC policy TXT record at `_dmarc.<domain>` (for example `v=DMARC1; p=quarantine; rua=mailto:dmarc@your-domain.example`) so receivers can enforce policy aligned with the signed `From` domain.
- Ensure the SMTP envelope sender (`MAIL FROM`) and header `From` values stay on the same domain—this repository already enforces that alignment for you.

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
- `POST /api/password-reset/request` – Initiate a password reset email for local accounts.
- `POST /api/password-reset/complete` – Finalize a password reset using a token from email.

---

## License

This project is in early development. Licensing will be determined prior to public release.

---

## Contact

For development updates or inquiries, please contact the project maintainer.
