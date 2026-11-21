#!/usr/bin/env bash
# Install the latest mvave-chocolate-tui release on Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/WilsonNet/mvave-chocolate-tui/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/WilsonNet/mvave-chocolate-tui/main/install.sh | bash -s -- --run
#
# Options:
#   --dir DIR        install into DIR (default: ~/.local/bin, or $MVAVE_INSTALL_DIR)
#   --version TAG    install a specific release, e.g. v0.1.0 (default: latest)
#   --run            start the TUI after installing; extra args after -- are passed on
#   --check          install, then print the binary version and exit
#   --skip-checksum  install even if the release has no checksums (not recommended)
#   -h, --help       show this help
#
# Environment:
#   MVAVE_INSTALL_DIR   default install directory
#   GITHUB_TOKEN        sent to the GitHub API if set (avoids rate limits)

set -euo pipefail

REPO="WilsonNet/mvave-chocolate-tui"
BIN="mvave-chocolate-tui"

dir="${MVAVE_INSTALL_DIR:-$HOME/.local/bin}"
tag=""
run=0
check=0
skip_checksum=0
passthru=()

die() {
	printf 'install: %s\n' "$*" >&2
	exit 1
}

usage() {
	sed -n '2,20p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true
}

while [ $# -gt 0 ]; do
	case "$1" in
	--dir | --install-dir)
		[ -n "${2:-}" ] || die "--dir needs a value"
		dir="$2"
		shift 2
		;;
	--dir=* | --install-dir=*)
		dir="${1#*=}"
		shift
		;;
	--version | --tag)
		[ -n "${2:-}" ] || die "--version needs a value"
		tag="$2"
		shift 2
		;;
	--version=* | --tag=*)
		tag="${1#*=}"
		shift
		;;
	--run)
		run=1
		shift
		;;
	--check)
		check=1
		shift
		;;
	--skip-checksum)
		skip_checksum=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	--)
		shift
		passthru=("$@")
		break
		;;
	*)
		die "unknown option: $1 (try --help)"
		;;
	esac
done

[ "$(uname -s)" = "Linux" ] ||
	die "this TUI talks to ALSA, so it runs on Linux only (found $(uname -s))"

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "no prebuilt binary for $(uname -m); build it with: go build ." ;;
esac

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

api_args=(-fsSL)
[ -n "${GITHUB_TOKEN:-}" ] && api_args+=(-H "Authorization: Bearer $GITHUB_TOKEN")

if [ -z "$tag" ]; then
	release="$(curl "${api_args[@]}" "https://api.github.com/repos/$REPO/releases/latest")" ||
		die "could not reach the GitHub API"
	# Works for both pretty-printed and single-line JSON.
	tag="$(printf '%s\n' "$release" | tr ',' '\n' |
		grep '"tag_name"' | head -n 1 |
		sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"
	[ -n "$tag" ] || die "no published release found; install from source with: go install"
fi

case "$tag" in
v*) ;;
*) tag="v$tag" ;;
esac
version="${tag#v}"
name="${BIN}_${version}_linux_${arch}"
asset="${name}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$asset"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'install: downloading %s\n' "$url" >&2
curl -fsSL -o "$tmp/$asset" "$url" || die "download failed: $url"

if [ "$skip_checksum" = 1 ]; then
	printf 'install: WARNING: checksum verification skipped by flag\n' >&2
else
	curl -fsSL -o "$tmp/checksums.txt" \
		"https://github.com/$REPO/releases/download/$tag/checksums.txt" \
		|| die "no checksums.txt for $tag; refusing to install unverified (use --skip-checksum to override)"
	sum="$(grep -E "  (\./)?${asset}\$" "$tmp/checksums.txt" 2>/dev/null | head -n 1 || true)"
	[ -n "$sum" ] || die "checksums.txt has no entry for $asset; refusing to install unverified"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$tmp" && printf '%s\n' "$sum" | sha256sum -c --quiet -) ||
			die "checksum mismatch for $asset"
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$tmp" && printf '%s\n' "$sum" | shasum -a 256 -c --quiet -) ||
			die "checksum mismatch for $asset"
	else
		die "no sha256 tool; install coreutils or use --skip-checksum"
	fi
	printf 'install: checksum ok\n' >&2
fi

tar -xzf "$tmp/$asset" -C "$tmp" || die "could not extract $asset"
src="$tmp/$name/$BIN"
[ -f "$src" ] || src="$(find "$tmp" -type f -name "$BIN" -perm -u+x | head -n 1)"
[ -n "${src:-}" ] && [ -f "$src" ] || die "the archive has no $BIN binary"

mkdir -p "$dir"
tmpbin="$dir/.$BIN.tmp.$$"
cp "$src" "$tmpbin"
chmod 0755 "$tmpbin"
mv -f "$tmpbin" "$dir/$BIN"
dest="$dir/$BIN"

printf 'install: installed %s %s\n' "$dest" "$( "$dest" --version 2>/dev/null || echo "" )" >&2

case ":$PATH:" in
*":$dir:"*) ;;
*)
	printf 'install: add %s to PATH, e.g.\n' "$dir" >&2
	printf '  export PATH="%s:$PATH"\n' "$dir" >&2
	;;
esac

if [ "$check" = 1 ]; then
	"$dest" --version
	exit 0
fi

if [ "$run" = 1 ]; then
	printf 'install: starting %s — q quits, --write enables writes\n' "$BIN" >&2
	if [ -t 0 ]; then
		exec "$dest" "${passthru[@]}"
	elif [ -r /dev/tty ]; then
		exec "$dest" "${passthru[@]}" </dev/tty
	else
		die "no terminal to open the TUI in; run it yourself: $dest"
	fi
fi

printf 'install: run it with: %s\n' "$dest" >&2
