#!/bin/bash
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
else
    DOCKER_COMPOSE="docker compose"
fi

sudo $DOCKER_COMPOSE down -v && sudo $DOCKER_COMPOSE up -d --build
