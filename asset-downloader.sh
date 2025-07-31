#!/bin/bash
set -e

mkdir -p frontend/static/css
mkdir -p frontend/static/js
mkdir -p frontend/static/webfonts
mkdir -p frontend/static/fonts

curl -sL "https://cdn.tailwindcss.com" -o "frontend/static/js/tailwindcss.js"

curl -sL "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/css/all.min.css" -o "frontend/static/css/all.min.css"
curl -sL "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-brands-400.woff2" -o "frontend/static/webfonts/fa-brands-400.woff2"
curl -sL "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-regular-400.woff2" -o "frontend/static/webfonts/fa-regular-400.woff2"
curl -sL "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-solid-900.woff2" -o "frontend/static/webfonts/fa-solid-900.woff2"
curl -sL "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-v4compatibility.woff2" -o "frontend/static/webfonts/fa-v4compatibility.woff2"

sed -i.bak 's|https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/|/static/webfonts/|g' frontend/static/css/all.min.css
rm frontend/static/css/all.min.css.bak

curl -sL "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" -A "Mozilla/5.0" -o "frontend/static/css/inter.css"

grep -o "https://fonts.gstatic.com/s/inter/[^)]*" frontend/static/css/inter.css | while read -r url; do
  filename=$(basename "$url")
  curl -sL "$url" -o "frontend/static/fonts/$filename"
done

sed -i.bak 's|https://fonts.gstatic.com/s/inter/v[0-9]*/|/static/fonts/|g' frontend/static/css/inter.css
rm frontend/static/css/inter.css.bak

echo "All assets downloaded successfully!"
