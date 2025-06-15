test:
    #!/usr/bin/env bash
    set -eu

    jsonnet-kit test

lint:
    #!/usr/bin/env bash
    set -eu

    golangci-lint run

build-dev:
    #!/usr/bin/env bash
    set -eu
    goreleaser release --snapshot --clean --config .goreleaser.dev.yaml

it: build-dev
    #!/usr/bin/env bash
    set -eu
    ./dist/darwin_darwin_arm64_v8.0/yokai it

dev:
    #!/usr/bin/env bash
    set -eu
    docker compose -f debug/docker-compose.yml up -d

check-git-state:
    #!/usr/bin/env bash
    set -eu
    
    # Check if we're on main branch
    if [[ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]]; then
        echo "Error: Must be on main branch"
        exit 1
    fi
    
    # Check if working directory is clean
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "Error: Working directory is not clean"
        exit 1
    fi
    
    # Check if we're in sync with origin/main
    git fetch origin
    if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
        echo "Error: Local main is not in sync with origin/main"
        exit 1
    fi

bump-major: check-git-state
    #!/usr/bin/env bash
    set -eu
    
    # Get the latest version tag
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    VERSION=${LATEST_TAG#v}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
    
    # Increment major version
    NEW_VERSION="v$((MAJOR + 1)).0.0"
    
    # Create and push the new tag
    git tag -a "$NEW_VERSION" -m "Bump version to $NEW_VERSION"
    git push origin "$NEW_VERSION"

bump-minor: check-git-state
    #!/usr/bin/env bash
    set -eu
    
    # Get the latest version tag
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    VERSION=${LATEST_TAG#v}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
    
    # Increment minor version
    NEW_VERSION="v${MAJOR}.$((MINOR + 1)).0"
    
    # Create and push the new tag
    git tag -a "$NEW_VERSION" -m "Bump version to $NEW_VERSION"
    git push origin "$NEW_VERSION"

bump-patch: check-git-state
    #!/usr/bin/env bash
    set -eu
    
    # Get the latest version tag
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    VERSION=${LATEST_TAG#v}
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
    
    # Increment patch version
    NEW_VERSION="v${MAJOR}.${MINOR}.$((PATCH + 1))"
    
    # Create and push the new tag
    git tag -a "$NEW_VERSION" -m "Bump version to $NEW_VERSION"
    git push origin "$NEW_VERSION"
