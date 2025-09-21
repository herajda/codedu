# ---- build ----
FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# ---- run ----
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY package*.json ./
RUN npm install --omit=dev
COPY --from=build /app/build ./build
EXPOSE 3000
ENV PORT=3000
ENV PUBLIC_API_BASE=/api
ENV SSR_API_BASE=http://backend:8080
CMD ["node", "build/index.js"]
