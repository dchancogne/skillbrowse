#!/bin/sh
# Installs the latest skillbrowse release for the current OS/arch.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dchancogne/skillbrowse/main/install.sh | sh
#
# Verifies the downloaded archive's SHA-256 checksum against the
# release's published checksums.txt before installing anything.
#
# This script does NOT verify the Ed25519 signature on checksums.txt —
# that check requires code (see internal/update/verify.go), and a shell
# script can't meaningfully bootstrap trust in itself before it has run.
# skillbrowse's own `upgrade` command performs full Ed25519 signature
# verification for every update after this first install. If you want
# to verify the signature on THIS install yourself before running
# anything, see "Verifying releases" in README.md and check manually.

set -eu

REPO="dchancogne/skillbrowse"
INSTALL_DIR="${SKILLBROWSE_INSTALL_DIR:-$HOME/.local/bin}"

main() {
	need_cmd curl
	need_cmd tar
	need_cmd shasum_or_sha256sum

	os="$(detect_os)"
	arch="$(detect_arch)"
	asset="skillbrowse_${os}_${arch}.tar.gz"

	tag="$(latest_tag)"
	if [ -z "$tag" ]; then
		err "could not determine the latest release tag"
	fi

	workdir="$(mktemp -d)"
	trap 'rm -rf "$workdir"' EXIT

	base_url="https://github.com/${REPO}/releases/download/${tag}"
	echo "Downloading skillbrowse ${tag} for ${os}/${arch}..."
	curl -fsSL -o "$workdir/$asset" "$base_url/$asset"
	curl -fsSL -o "$workdir/checksums.txt" "$base_url/checksums.txt"

	verify_checksum "$workdir" "$asset"

	tar -xzf "$workdir/$asset" -C "$workdir"

	mkdir -p "$INSTALL_DIR"
	mv "$workdir/skillbrowse" "$INSTALL_DIR/skillbrowse"
	chmod +x "$INSTALL_DIR/skillbrowse"

	echo "Installed to $INSTALL_DIR/skillbrowse"
	case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) echo "Note: $INSTALL_DIR is not on your PATH. Add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	esac
	"$INSTALL_DIR/skillbrowse" version
}

detect_os() {
	case "$(uname -s)" in
	Darwin) echo "darwin" ;;
	Linux) echo "linux" ;;
	*) err "unsupported OS: $(uname -s) (skillbrowse supports macOS and Linux)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo "amd64" ;;
	arm64 | aarch64) echo "arm64" ;;
	*) err "unsupported architecture: $(uname -m)" ;;
	esac
}

latest_tag() {
	curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
}

verify_checksum() {
	dir="$1"
	asset="$2"
	want="$(grep " ${asset}\$" "$dir/checksums.txt" | awk '{print $1}')"
	if [ -z "$want" ]; then
		err "checksums.txt does not list $asset"
	fi

	if command -v shasum >/dev/null 2>&1; then
		got="$(shasum -a 256 "$dir/$asset" | awk '{print $1}')"
	else
		got="$(sha256sum "$dir/$asset" | awk '{print $1}')"
	fi

	if [ "$want" != "$got" ]; then
		err "checksum mismatch for $asset: expected $want, got $got"
	fi
	echo "Checksum verified."
}

need_cmd() {
	if [ "$1" = "shasum_or_sha256sum" ]; then
		if ! command -v shasum >/dev/null 2>&1 && ! command -v sha256sum >/dev/null 2>&1; then
			err "need shasum or sha256sum (neither found)"
		fi
		return
	fi
	if ! command -v "$1" >/dev/null 2>&1; then
		err "need '$1' (command not found)"
	fi
}

err() {
	echo "install.sh: $*" >&2
	exit 1
}

main "$@"
