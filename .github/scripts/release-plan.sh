#!/usr/bin/env bash
# Decides whether a push should publish a release, from CHANGELOG.md.
#
#   release-plan.sh plan         print release=, tag=, create_tag= lines for $GITHUB_OUTPUT
#   release-plan.sh notes X.Y.Z  print the CHANGELOG section for X.Y.Z (the release notes)
#
# On a push to the default branch, the newest versioned section of
# CHANGELOG.md ("## [X.Y.Z] - date") is released if tag vX.Y.Z does not exist
# yet. It must be newer than every existing tag, so a typo cannot publish.
# On a pushed tag vX.Y.Z, CHANGELOG.md must have a section for X.Y.Z.
set -euo pipefail

changelog="${CHANGELOG:-CHANGELOG.md}"

# newest_version prints the first "## [X.Y.Z]" heading, skipping [Unreleased].
newest_version() {
	sed -nE 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/p' "$changelog" | head -n1
}

# notes prints the body of the section for version $1, without its heading.
notes() {
	awk -v v="$1" '
		/^## \[/ { if (found) exit; if (index($0, "## [" v "]") == 1) { found = 1; next } }
		found { print }
		END { if (!found) exit 1 }
	' "$changelog" | sed -e '/./,$!d'
}

latest_tag() {
	git tag --list 'v[0-9]*.[0-9]*.[0-9]*' | sed 's/^v//' | sort -V | tail -n1
}

# newer A B: true when version A sorts strictly after version B.
newer() {
	[ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n1)" = "$1" ]
}

plan() {
	if [[ "${GITHUB_REF:-}" == refs/tags/v* ]]; then
		local version="${GITHUB_REF#refs/tags/v}"
		if ! notes "$version" >/dev/null; then
			echo "::error::CHANGELOG.md has no section for $version; add \"## [$version] - $(date -u +%F)\" before tagging" >&2
			exit 1
		fi
		printf 'release=true\ntag=v%s\ncreate_tag=false\n' "$version"
		return
	fi

	local version
	version="$(newest_version)"
	if [ -z "$version" ]; then
		echo "release=false"
		return
	fi
	if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
		echo "v$version is already released; nothing to do" >&2
		echo "release=false"
		return
	fi
	local latest
	latest="$(latest_tag)"
	if [ -n "$latest" ] && ! newer "$version" "$latest"; then
		echo "::error::CHANGELOG.md's newest version $version is not newer than the latest tag v$latest" >&2
		exit 1
	fi
	if [ -z "$(notes "$version" | tr -d '[:space:]')" ]; then
		echo "::error::CHANGELOG.md section $version is empty" >&2
		exit 1
	fi
	echo "Releasing v$version (latest tag: v${latest:-none})" >&2
	printf 'release=true\ntag=v%s\ncreate_tag=true\n' "$version"
}

case "${1:-}" in
plan) plan ;;
notes) notes "${2:?usage: $0 notes X.Y.Z}" ;;
*)
	echo "usage: $0 plan | notes X.Y.Z" >&2
	exit 2
	;;
esac
