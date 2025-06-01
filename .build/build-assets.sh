#!/bin/bash
set -e
export NODE_OPTIONS="--max-old-space-size=8192"

# This script is used to build the assets for the application.
cd assets
rm -rf build
bun install
bunx yarn version --new-version $1 --no-git-tag-version
bun run build

# Copy the build files to the application directory
cd ../
zip -r - assets/build >assets.zip
mv assets.zip application/statics