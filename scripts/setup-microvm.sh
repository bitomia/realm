#!/usr/bin/env bash

set -xeuo pipefail

command -v make > /dev/null 2>&1 || { echo  "need build deps: build-essential libelf-dev libssl-dev" >&2; exit 1; }
command -v gcc > /dev/null 2>&1 || { echo  "need build deps: build-essential libelf-dev libssl-dev">&2; exit 1; }
command -v bison > /dev/null 2>&1 || { echo  "need build deps: bison" >&2; exit 1; }
command -v flex > /dev/null 2>&1 || { echo  "need build deps: flex" >&2; exit 1; }

KERNEL="linux-6.1.186"
KERNEL_CONFIG="microvm-kernel-ci-x86_64-6.1.config"
OUT_DIR=$(pwd)

TEMP_DIR=$(mktemp -d)
pushd $TEMP_DIR

curl -fsSL --progress-bar https://cdn.kernel.org/pub/linux/kernel/v6.x/$KERNEL.tar.xz | tar xJ
curl -fsSL -o "$KERNEL/.config" https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/resources/guest_configs/$KERNEL_CONFIG

make -C $KERNEL olddefconfig
make -C $KERNEL vmlinux -j"$(nproc)"
cp "$KERNEL/vmlinux" $OUT_DIR
popd

rm -rf $TEMP_DIR
echo "Kernel ready: $OUT_DIR/vmlinux"
