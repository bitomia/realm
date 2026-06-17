#!/bin/sh
set -e

REPO="bitomia/realm"
INSTALL_DIR="${REALM_INSTALL_DIR:-/usr/local/bin}"

main() {
    os="$(detect_os)"
    arch="$(detect_arch)"

    if [ -z "$os" ] || [ -z "$arch" ]; then
        echo "Error: unsupported platform: $(uname -s)/$(uname -m)" >&2
        exit 1
    fi

    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
        echo "Error: cannot create ${INSTALL_DIR} (permission denied)" >&2
        echo "Run with sudo or set REALM_INSTALL_DIR to a writable location" >&2
        exit 1
    fi
    if [ ! -w "$INSTALL_DIR" ]; then
        echo "Error: no write permission for ${INSTALL_DIR}" >&2
        echo "Run with sudo or set REALM_INSTALL_DIR to a writable location" >&2
        exit 1
    fi

    tag="$(get_latest_tag)"
    if [ -z "$tag" ]; then
        echo "Error: could not determine latest release" >&2
        exit 1
    fi
    echo "Installing ${tag}..."

    asset_name="realm-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        asset_name="${asset_name}.zip"
    else
        asset_name="${asset_name}.tar.gz"
    fi
    url="https://github.com/${REPO}/releases/download/${tag}/${asset_name}"
    checksum_url="${url}.sha256"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    echo "Downloading ${asset_name}..."
    download "$url" "${tmpdir}/${asset_name}"
    download "$checksum_url" "${tmpdir}/${asset_name}.sha256"

    echo "Verifying checksum..."
    verify_checksum "${tmpdir}" "${asset_name}"

    echo "Extracting to ${INSTALL_DIR}..."
    if [ "$os" = "windows" ]; then
        unzip -o "${tmpdir}/${asset_name}" -d "$INSTALL_DIR"
    else
        tar -xzf "${tmpdir}/${asset_name}" -C "$INSTALL_DIR"
    fi

    binary="realm"
    if [ "$os" = "windows" ]; then
        binary="realm.exe"
    fi
    chmod +x "${INSTALL_DIR}/${binary}"

    echo "realm ${tag} installed to ${INSTALL_DIR}/${binary}"

    if [ "$os" = "linux" ]; then
        check_container_deps
        check_vmm_deps
    fi

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        echo ""
        echo "Add ${INSTALL_DIR} to your PATH:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) return 1 ;;
    esac
}

detect_arch() {
    case "$(uname -s)" in
        Darwin*) echo "universal" ;;
        *)
            case "$(uname -m)" in
                x86_64|amd64) echo "amd64" ;;
                aarch64|arm64) echo "arm64" ;;
                *) return 1 ;;
            esac
            ;;
    esac
}

get_latest_tag() {
     if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o /dev/null -w '%{url_effective}\n' \
            "https://github.com/${REPO}/releases/latest" |
            sed 's|.*/tag/||'
    else
        echo "Error: curl is required" >&2
        exit 1
    fi
}

download() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$dest" "$url"
    fi
}

check_container_deps() {
    missing=""

    if ! command -v containerd >/dev/null 2>&1; then
        missing="containerd"
    fi

    cni_found=""
    for dir in /opt/cni/bin /usr/lib/cni /usr/libexec/cni; do
        if [ -d "$dir" ] && [ -x "${dir}/bridge" ]; then
            cni_found="yes"
            break
        fi
    done
    if [ -z "$cni_found" ]; then
        missing="${missing:+${missing} }containernetworking-plugins"
    fi

    if [ -n "$missing" ]; then
        echo "" >&2
        echo "Warning: the container engine will not be available." >&2
        echo "Missing dependencies: ${missing}" >&2
        echo "Install them to enable container support, e.g.:" >&2
        echo "  Debian/Ubuntu: sudo apt-get install ${missing}" >&2
        echo "  Fedora/RHEL:   sudo dnf install ${missing}" >&2
    fi
}

check_vmm_deps() {
    missing=""

    if ! command -v libvirtd >/dev/null 2>&1 && ! command -v virsh >/dev/null 2>&1; then
        missing="libvirt"
    fi

    qemu_found=""
    for bin in qemu-system-x86_64 qemu-system-aarch64 qemu-kvm; do
        if command -v "$bin" >/dev/null 2>&1; then
            qemu_found="yes"
            break
        fi
    done
    if [ -z "$qemu_found" ]; then
        missing="${missing:+${missing} }qemu"
    fi

    if [ -n "$missing" ]; then
        echo "" >&2
        echo "Warning: virtual machines (VMMs) will not be available." >&2
        echo "Missing dependencies: ${missing}" >&2
        echo "Install them to enable VM support, e.g.:" >&2
        echo "  Debian/Ubuntu: sudo apt-get install libvirt-daemon-system bridge-utils qemu-system" >&2
        echo "  Fedora/RHEL:   sudo dnf install libvirt qemu-kvm bridge-utils" >&2
    fi
}

verify_checksum() {
    dir="$1"
    file="$2"
    expected="$(awk '{print $1}' "${dir}/${file}.sha256")"
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "${dir}/${file}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "${dir}/${file}" | awk '{print $1}')"
    else
        echo "Warning: no sha256 tool found, skipping checksum verification" >&2
        return 0
    fi
    if [ "$expected" != "$actual" ]; then
        echo "Error: checksum mismatch" >&2
        echo "  expected: ${expected}" >&2
        echo "  actual:   ${actual}" >&2
        exit 1
    fi
}

main
