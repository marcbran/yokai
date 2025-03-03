
docker-build name suffix="" dockerfile="./Dockerfile" context=".":
    #!/usr/bin/env bash
    set -eu

    NAME="{{ name }}"
    SUFFIX={{ suffix }}
    DOCKERFILE={{ dockerfile }}
    CONTEXT={{ context }}

    TAG="$(git rev-parse --short HEAD)"
    BRANCH="$(git rev-parse --abbrev-ref HEAD)"

    docker build -t "${NAME}${SUFFIX}:${TAG}" -f "${DOCKERFILE}" "${CONTEXT}"
    docker tag "${NAME}${SUFFIX}:${TAG}" "${NAME}${SUFFIX}:${BRANCH}"

build: (docker-build "yokai")

it: build
    #!/usr/bin/env bash
    set -eu
    cd it
    just run
