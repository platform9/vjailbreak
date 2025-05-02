# Use a tiny web server base image
FROM nginx:alpine

# Copy generated Swagger UI files
COPY swagger-ui/ /usr/share/nginx/html

# Expose port (optional if using ingress)
EXPOSE 80

# nginx will serve /index.html by default
