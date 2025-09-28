# ---- build ----
FROM golang:1.23 AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build package (all files), output to /out
RUN mkdir -p /out \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/codegrader .

# ---- run ----
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        tzdata \
        bash \
        docker.io \
        python3 \
        python3-venv \
        python3-pip \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/codegrader /usr/local/bin/codegrader
COPY schema.sql ./schema.sql
COPY llm_agent /app/llm_agent
COPY scripts /app/scripts
# Install agent dependencies into an isolated virtualenv to satisfy PEP 668.
RUN python3 -m venv /opt/llm-agent-venv \
    && /opt/llm-agent-venv/bin/pip install --no-cache-dir -r /app/llm_agent/requirements.txt \
    && find /app/scripts -type f -name "*.py" -exec chmod +x {} +
RUN mkdir -p /uploads /sandbox
ENV PORT=8080
ENV EXECUTION_ROOT=/sandbox
ENV PYTHON_RUNNER_IMAGE=python:3.11
ENV DOCKER_HOST=tcp://docker-engine:2375
ENV DOCKER_TLS_CERTDIR=
ENV LLM_EVALUATOR_PYTHON=/opt/llm-agent-venv/bin/python
EXPOSE 8080
CMD ["codegrader"]
