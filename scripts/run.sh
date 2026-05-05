#!/usr/bin/env bash

COMMAND=${1:-"start"}

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"

if [ -f "${ENV_FILE}" ]; then
    set -a
    # shellcheck disable=SC1090
    . "${ENV_FILE}"
    set +a
fi

export PHYSIO_API_ENVIRONMENT="${PHYSIO_API_ENVIRONMENT:-Development}"
export PHYSIO_API_PORT="${PHYSIO_API_PORT:-8080}"
export PHYSIO_API_MONGODB_HOST="${PHYSIO_API_MONGODB_HOST:-localhost}"
export PHYSIO_API_MONGODB_PORT="${PHYSIO_API_MONGODB_PORT:-27017}"
export PHYSIO_API_MONGODB_DATABASE="${PHYSIO_API_MONGODB_DATABASE:-wac-physio}"
export PHYSIO_API_MONGODB_TIMEOUT_SECONDS="${PHYSIO_API_MONGODB_TIMEOUT_SECONDS:-10}"
export DOCKER_HUB_ID="meowiky002"

mongo() {
    docker compose --env-file "${ENV_FILE}" --file "${PROJECT_ROOT}/deployments/docker-compose/compose.yaml" "$@"
}

cleanup() {
    mongo down
}

case "$COMMAND" in
    start)
        trap cleanup EXIT
        mongo up --detach
        go run "${PROJECT_ROOT}/cmd/api-service"
        ;;
    openapi)
        docker run --rm -ti \
            -v "${PROJECT_ROOT}:/local" \
            openapitools/openapi-generator-cli \
            generate -c /local/scripts/generator-cfg.yaml
        ;;
    test)
        (
            cd "${PROJECT_ROOT}" || exit 1
            go test -v ./...
        )
        ;;
    mongo)
        mongo up
        ;;
    # docker)
    #     (
    #         cd "${PROJECT_ROOT}" || exit 1
    #         docker build -t "${DOCKER_HUB_ID}/ambulance-wl-webapi:local-build" -f "${PROJECT_ROOT}/build/docker/Dockerfile" .
    #     )
    #     ;;
    *)
        echo "Unknown command: $COMMAND" >&2
        exit 1
        ;;
esac
