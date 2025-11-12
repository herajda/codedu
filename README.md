# CodEdu

CodEdu is a modern platform for classrooms and training programs to assign coding tasks, collect submissions, and deliver instant feedback. It streamlines the full workflow for teachers and students—from creating assignments to automated evaluation and clear, actionable results.

—

## Highlights
- Role‑based access for students, teachers, and admins
- Easy assignment creation and management
- One‑click code submissions with immediate feedback
- Clear grading and progress tracking
- Secure, self‑hosted by you

—

## Quick Start

Prerequisites: Docker and Docker Compose installed on your machine/server.

1) Start the database (first time only)
- `docker compose -f docker-compose.db.yml up -d`

2) Launch the app
- `docker compose up -d`

3) Open the app
- Visit `http://localhost:8080`

Stop the app at any time with `docker compose down`. To stop the database as well: `docker compose -f docker-compose.db.yml down`.

—

## Configuration

To customize your instance (admin account, email, captchas, integrations), set environment variables in a `.env` file before starting. A copy of example values is provided in the repository. Restart the containers after changes.

—

## Environment

Create a `.env` file at the project root before starting the app. At minimum, set the following variables:

- `DATABASE_URL` – PostgreSQL connection string
- `JWT_SECRET` – secret used to sign sessions/tokens
- `ADMIN_EMAIL` – initial administrator email
- `ADMIN_PASSWORD` – initial administrator password

Optional (recommended for production):

- `PUBLIC_TURNSTILE_SITE_KEY` – site key for Turnstile challenge (frontend)
- `TURNSTILE_SECRET_KEY` – secret key for Turnstile verification (backend)
- `PASSWORD_RESET_BASE_URL` – public URL of the app, used in emails (e.g., `https://codedu.example.com`)
- `APP_BASE_URL` – optional; overrides links in emails (defaults to `PASSWORD_RESET_BASE_URL`)
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD` – SMTP credentials for outgoing email
- `SMTP_FROM` – verified sender address (e.g., `no-reply@your-domain.com`)
- `SMTP_FROM_NAME` – display name for the sender
- `SMTP_DKIM_SELECTOR`, `SMTP_DKIM_DOMAIN` – DKIM signing configuration (optional)
- `SMTP_DKIM_PRIVATE_KEY` or `SMTP_DKIM_PRIVATE_KEY_FILE` – DKIM private key (inline or file path)
- `BAKALARI_BASE_URL` – optional integration endpoint for Bakaláři

Quick start template:

```
DATABASE_URL=postgres://postgres:postgres@db:5432/codedu?sslmode=disable
JWT_SECRET=change-me-very-secret

ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me-strong

# Public URL of your deployment (used for links in emails)
PASSWORD_RESET_BASE_URL=http://localhost:8080

# Email (optional but recommended)
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=no-reply@example.com
SMTP_FROM_NAME=CodEdu

# Human verification (optional)
PUBLIC_TURNSTILE_SITE_KEY=
TURNSTILE_SECRET_KEY=

# Optional integrations
BAKALARI_BASE_URL=
```

After editing `.env`, (re)start the services with `docker compose up -d`.

—

## Backup & Restore
- Automated periodic backups of the database are created and stored locally in the `backups` folder.
- To restore from a backup, stop the app, restore the database from the desired archive, and start the app again.

—

## Support & Feedback
- Found an issue or have a feature request? Open an issue in this repository.
- For general questions, please get in touch with your system administrator or course lead.

—

## License

---

## Contact

For development updates or inquiries, please contact the project maintainer.
