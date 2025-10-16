#!/bin/sh

# Generate a secure, random nonce
export CSP_NONCE=$(openssl rand -base64 16)

# Substitute the nonce
envsubst '${CSP_NONCE}' < /etc/nginx/conf.d/default.conf > /etc/nginx/conf.d/default.conf.temp
mv -f /etc/nginx/conf.d/default.conf.temp /etc/nginx/conf.d/default.conf

# Inject the nonce
sed -i "s|<script type=\"module\"|<script nonce=\"${CSP_NONCE}\" type=\"module\"|g" /usr/share/nginx/html/index.html

# Use envsubst to substitute environment variables in the nginx configuration
VITE_API_TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token) envsubst < /usr/share/nginx/html/index.html > /usr/share/nginx/html/index.html.temp
mv -f /usr/share/nginx/html/index.html.temp /usr/share/nginx/html/index.html

# Start Nginx
nginx -g 'daemon off;'