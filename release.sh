#!/bin/bash

set -o errexit -o nounset -o pipefail
set -xv

dryrun='false'

print_usage() {
  printf "Usage: TODO..."
}

while getopts 'd' flag; do
  case "${flag}" in
    d) dryrun='true' ;;
    *) print_usage
       exit 1 ;;
  esac
done


echo "dryrun is $dryrun"
echo "\$@ pre shift is \"$@\""
shift $((OPTIND - 1))
echo "\$@ post shift is \"$@\""

TAG="$@"
echo "TAG is $TAG"

goreleaser check
if [ $dryrun = "true" ]; then
  goreleaser --rm-dist --snapshot
  # goreleaser --rm-dist --skip-publish
else
  git tag $TAG
  git push origin $TAG
  goreleaser release --rm-dist
fi
