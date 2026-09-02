#!/usr/bin/env bash
#
# Builds a raw ext4 rootfs for the firecracker node driver out of a container base image
#   Usage example:  INSTALL_PACKAGES=0 BASE_IMAGE=hello-world ./setup-microvm-rootfs.sh 
#

set -xeuo pipefail

RUNTIME="${RUNTIME:-}"
if [ -z "$RUNTIME" ]; then
  if command -v docker > /dev/null 2>&1; then
    RUNTIME="docker"
  elif command -v podman > /dev/null 2>&1; then
    RUNTIME="podman"
  else
    echo "need a container runtime: docker or podman" >&2
    exit 1
  fi
fi

MKFS="/sbin/mkfs.ext4"

[ -n "$MKFS" ] || { echo "need build deps: e2fsprogs" >&2; exit 1; }
command -v truncate > /dev/null 2>&1 || { echo "need build deps: coreutils" >&2; exit 1; }
command -v go > /dev/null 2>&1 || { echo "need build deps: go, to build the guest init" >&2; exit 1; }

SUDO="sudo"
if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
elif ! command -v sudo > /dev/null 2>&1; then
  echo "need root, or sudo, to preserve file ownership in the image" >&2
  exit 1
fi

BASE_IMAGE="${BASE_IMAGE:-ubuntu:24.04}"
SIZE="${SIZE:-2G}"
OUT_DIR=$(pwd)
OUT_FILE="${OUT_FILE:-$OUT_DIR/rootfs.ext4}"
INSTALL_PACKAGES="${INSTALL_PACKAGES:-1}"

REPO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

PACKAGES=(
  systemd            # the init itself
  systemd-sysv       # provides /sbin/init, the path the kernel looks for
  udev
  dbus
  e2fsprogs          # resize2fs, likewise
  openssh-server
  iproute2
  iputils-ping
  ca-certificates
)

BUILD_TAG="realm-microvm-rootfs:${BASE_IMAGE//[^a-zA-Z0-9._-]/-}"

CID=""
TEMP_DIR=$(mktemp -d)
cleanup() {
  if [ -n "$CID" ]; then
    $RUNTIME rm -f "$CID" > /dev/null 2>&1 || true
  fi
  if [ -d "$TEMP_DIR/rootfs" ]; then
    $SUDO rm -rf "$TEMP_DIR"
  else
    rm -rf "$TEMP_DIR"
  fi
}
trap cleanup EXIT
pushd "$TEMP_DIR"

CGO_ENABLED=0 go build -C "$REPO_DIR" -o "$TEMP_DIR/init" ./drivers/loads/microvm/init.go

cat > Dockerfile <<EOF
FROM $BASE_IMAGE
COPY init /init
EOF

if [ "$INSTALL_PACKAGES" = "1" ]; then
  cat >> Dockerfile <<EOF
RUN apt-get update \\
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ${PACKAGES[*]} \\
 && rm -rf /var/lib/apt/lists/*
# An empty root password, so the serial console gives a usable login on a
# microVM that has no keys provisioned yet
RUN passwd -d root
EOF
fi

$RUNTIME build -t "$BUILD_TAG" .

CID=$($RUNTIME create "$BUILD_TAG" /bin/true)
ROOTFS_DIR="$TEMP_DIR/rootfs"
$SUDO mkdir -p "$ROOTFS_DIR"
$RUNTIME export "$CID" | $SUDO tar -C "$ROOTFS_DIR" -xf -
$SUDO rm -f "$ROOTFS_DIR/.dockerenv"

# `docker export` flattens device nodes into empty regular files. The kernel
# opens /dev/console for PID 1, so without real nodes the guest boots with no
# stdout at all and the console stays silent
$SUDO mkdir -p "$ROOTFS_DIR/dev"
$SUDO rm -f "$ROOTFS_DIR/dev/console" "$ROOTFS_DIR/dev/null"
$SUDO mknod -m 600 "$ROOTFS_DIR/dev/console" c 5 1
$SUDO mknod -m 666 "$ROOTFS_DIR/dev/null" c 1 3

rm -f "$OUT_FILE"
truncate -s "$SIZE" "$OUT_FILE"
$SUDO "$MKFS" -F -L rootfs -d "$ROOTFS_DIR" "$OUT_FILE"
$SUDO chown "$(id -u):$(id -g)" "$OUT_FILE"

popd

set +x
echo "Rootfs ready: $OUT_FILE"
