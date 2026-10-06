#!/bin/sh
# Runs from the iOS "Bundle React Native code and images" build phase (via
# with-environment.sh, which sets NODE_BINARY). Maps the Xcode configuration
# to an Atlas environment; see "Environments" in atlas-mobile/README.md.
#   Debug   -> $ATLAS_ENV, or local when unset
#   Staging -> staging (TODO(T18): the Staging configuration is not created yet)
#   Release -> prod
set -e

case "$CONFIGURATION" in
  Release) set -- --env prod ;;
  Staging) set -- --env staging ;;
  *) set -- ;;
esac

cd "$(dirname "$0")/.."
"${NODE_BINARY:-node}" scripts/generate-env.js "$@"
