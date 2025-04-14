FROM node:18-alpine

# Set work directory
WORKDIR /app

# Copy files
COPY package*.json ./
RUN npm install

COPY . .

# Start the app
CMD ["npm", "start"]
