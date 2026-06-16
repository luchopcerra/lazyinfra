#!/usr/bin/env sh
set -eu

REPO="luchopcerra/lazyinfra"
BINARY_NAME="lazyinfra"
INSTALL_DIR="/usr/local/bin"

fail() {
  echo "lazyinfra install error: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) echo "darwin" ;;
    linux) echo "linux" ;;
    *) fail "unsupported operating system: $os" ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64 | amd64) echo "amd64" ;;
    arm64 | aarch64) echo "arm64" ;;
    *) fail "unsupported architecture: $arch" ;;
  esac
}

download() {
  url="$1"
  output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --retry-delay 2 "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$output" "$url"
  else
    fail "curl or wget is required to download lazyinfra"
  fi
}

api_get() {
  url="$1"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
  else
    fail "curl or wget is required to query GitHub releases"
  fi
}

need_cmd uname
need_cmd mktemp
need_cmd tar

os="$(detect_os)"
arch="$(detect_arch)"

echo "Detecting latest lazyinfra release..."
release_json="$(api_get "https://api.github.com/repos/${REPO}/releases/latest")"
tag="$(printf '%s' "$release_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
[ -n "$tag" ] || fail "could not determine the latest release tag from GitHub"

version="${tag#v}"
archive="${BINARY_NAME}_${version}_${os}_${arch}.tar.gz"
download_url="https://github.com/${REPO}/releases/download/${tag}/${archive}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

echo "Downloading ${archive}..."
if ! download "$download_url" "$tmp_dir/$archive"; then
  fail "release asset not found for ${os}/${arch}: ${archive}"
fi

echo "Extracting archive..."
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

binary_path="$(find "$tmp_dir" -type f -name "$BINARY_NAME" | head -n 1)"
[ -n "$binary_path" ] || fail "archive did not contain a ${BINARY_NAME} executable"

chmod +x "$binary_path"
target="${INSTALL_DIR}/${BINARY_NAME}"

echo "Installing to ${target}..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$binary_path" "$target"
else
  need_cmd sudo
  sudo mv "$binary_path" "$target"
fi

echo "lazyinfra ${tag} installed successfully."
echo "Run '${BINARY_NAME} --help' to get started."
