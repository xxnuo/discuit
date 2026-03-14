#!/bin/bash
set -e

go build

cd ui
pnpm install --frozen-lockfile
pnpm build
cd ..
