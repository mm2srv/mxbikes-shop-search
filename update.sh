#!/bin/bash
set -e

go run -v main.go

if git status | grep mxbikes-shop-tracks.json > /dev/null; then
  git add mxbikes-shop-tracks.json processed_tracks.json
  git commit -m "update tracks"
  git push origin main:main
  exit 0
else 
  echo "NO NEW TRACKS"
  exit 0
fi
