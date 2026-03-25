#!/bin/sh
# Inari install script
# Usage: curl -fsSL https://raw.githubusercontent.com/KilimcininKorOglu/inari/main/install.sh | sh
set -e

REPO="KilimcininKorOglu/inari"
BINARY="inari"

# Determine OS and architecture (Go naming convention)
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "${OS}" in
        Linux)
            GOOS="linux"
            ;;
        Darwin)
            GOOS="darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            GOOS="windows"
            ;;
        *)
            echo "Error: unsupported operating system: ${OS}" >&2
            exit 1
            ;;
    esac

    case "${ARCH}" in
        x86_64|amd64)
            GOARCH="amd64"
            ;;
        arm64|aarch64)
            GOARCH="arm64"
            ;;
        *)
            echo "Error: unsupported architecture: ${ARCH}" >&2
            exit 1
            ;;
    esac
}

# Get the latest release tag from GitHub
get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [ -z "${VERSION}" ]; then
        echo "Error: could not determine latest release version" >&2
        exit 1
    fi
}

# Choose install directory
choose_install_dir() {
    if [ -d "${HOME}/.local/bin" ]; then
        INSTALL_DIR="${HOME}/.local/bin"
    elif [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "${INSTALL_DIR}"
    fi
}

# Download and install the binary
install() {
    detect_platform
    get_latest_version
    choose_install_dir

    EXT=""
    if [ "${GOOS}" = "windows" ]; then
        EXT=".exe"
    fi

    FILENAME="${BINARY}-${VERSION}-${GOOS}-${GOARCH}${EXT}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    echo "Installing Inari ${VERSION} for ${GOOS}-${GOARCH}..."
    echo "  Downloading from ${URL}"

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "${TMP_DIR}"' EXIT

    curl -fsSL "${URL}" -o "${TMP_DIR}/${BINARY}${EXT}"
    chmod +x "${TMP_DIR}/${BINARY}${EXT}"
    mv "${TMP_DIR}/${BINARY}${EXT}" "${INSTALL_DIR}/${BINARY}${EXT}"

    echo "  Installed to ${INSTALL_DIR}/${BINARY}${EXT}"
    echo ""

    # Check if install dir is in PATH
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            echo "Note: ${INSTALL_DIR} is not in your PATH."
            echo "Add it with:"
            echo ""
            echo "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
            echo ""
            ;;
    esac

    echo "Run 'inari --help' to get started."
}

install
