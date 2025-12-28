#!/bin/bash
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
else
    DOCKER_COMPOSE="docker compose"
fi

$DOCKER_COMPOSE --env-file .env -f docker-compose.yml -f docker-compose.db.yml down -v \
  && $DOCKER_COMPOSE --env-file .env -f docker-compose.yml -f docker-compose.db.yml up -d --build
