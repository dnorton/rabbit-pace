#!/bin/sh
# Install rabbit, the running pace calculator, on Linux or macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/dnorton/rabbit-pace/main/install.sh | sh
#
# Environment variables:
#   RABBIT_VERSION      release tag to install, e.g. v0.1.0 (default: latest)
#   RABBIT_INSTALL_DIR  directory to install into (default: ~/.local/bin)
#
# For a system-wide install:
#   curl -fsSL .../install.sh | sudo RABBIT_INSTALL_DIR=/usr/local/bin sh
#
# The whole script is wrapped in main(), which is only called on the last
# line, so a download cut off halfway can't run a partial script.

set -eu

REPO="dnorton/rabbit-pace"

say() {
	printf 'rabbit-install: %s\n' "$1"
}

fail() {
	printf 'rabbit-install: error: %s\n' "$1" >&2
	exit 1
}

# download URL FILE fetches URL into FILE with curl or wget.
download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		fail "curl or wget is required"
	fi
}

# sha256 FILE prints the SHA-256 checksum of FILE.
sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		fail "sha256sum or shasum is required to verify the download"
	fi
}

main() {
	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "unsupported OS $(uname -s); on Windows, download the .zip from https://github.com/$REPO/releases" ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported CPU architecture $(uname -m)" ;;
	esac

	version="${RABBIT_VERSION:-latest}"
	if [ "$version" = latest ]; then
		base_url="https://github.com/$REPO/releases/latest/download"
	else
		base_url="https://github.com/$REPO/releases/download/$version"
	fi
	# RABBIT_BASE_URL is only for testing against a local build.
	base_url="${RABBIT_BASE_URL:-$base_url}"
	install_dir="${RABBIT_INSTALL_DIR:-$HOME/.local/bin}"
	archive="rabbit_${os}_${arch}.tar.gz"

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	say "downloading $archive ($version)"
	download "$base_url/$archive" "$tmp/$archive" || fail "could not download $base_url/$archive"
	download "$base_url/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt"

	expected="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
	[ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
	actual="$(sha256 "$tmp/$archive")"
	[ "$expected" = "$actual" ] || fail "checksum mismatch for $archive (expected $expected, got $actual)"
	say "checksum verified"

	tar -xzf "$tmp/$archive" -C "$tmp" rabbit

	mkdir -p "$install_dir" || fail "could not create $install_dir"
	# Copy next to the target and rename, so an existing rabbit is
	# replaced in one step even if it's currently running.
	cp "$tmp/rabbit" "$install_dir/.rabbit.new" || fail "could not write to $install_dir (try RABBIT_INSTALL_DIR or sudo)"
	chmod 755 "$install_dir/.rabbit.new"
	mv -f "$install_dir/.rabbit.new" "$install_dir/rabbit"

	say "installed $("$install_dir/rabbit" version) to $install_dir/rabbit"

	case ":$PATH:" in
	*":$install_dir:"*) ;;
	*)
		say "$install_dir is not on your PATH; add this to your shell profile:"
		# $PATH is meant literally here: it's text for the user to copy.
		# shellcheck disable=SC2016
		printf '\n    export PATH="%s:$PATH"\n\n' "$install_dir"
		;;
	esac
}

main "$@"
