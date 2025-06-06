# Use an official Node.js runtime as the base image
FROM node:22 AS builder

# Set the working directory in the container
WORKDIR /usr/src/app

# Copy package.json and package-lock.json to the working directory
COPY package*.json ./

# Install app dependencies
RUN yarn install

# Copy the rest of the app source code to the working directory
COPY src ./src
COPY index.html .
COPY public .
COPY vite* .
COPY tsconfig.json .

RUN yarn build

# Use a lightweight Nginx image to serve the built app
FROM nginx:1.24-alpine3.17-slim

# Copy built assets from the build stage
COPY --from=builder /usr/src/app/dist /usr/share/nginx/html

# Copy the public directory
COPY public /usr/share/nginx/html

# Copy the Nginx configuration file
COPY default.conf /etc/nginx/conf.d/

# Copy the startup script
COPY startup.sh /usr/local/bin/start.sh

# Expose port
EXPOSE 3000

# Start Nginx server
CMD ["/usr/local/bin/start.sh"]