# Stage 1: Build frontend
FROM node:20-slim AS frontend
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY packages/ packages/
COPY apps/ apps/
RUN pnpm install --frozen-lockfile && pnpm -r build

# Stage 2: Build backend
FROM golang:1.23 AS backend
WORKDIR /app
COPY backend/ ./
RUN go mod download && CGO_ENABLED=0 go build -o server ./cmd/server

# Stage 3: Production image
FROM gcr.io/distroless/static-debian12
COPY --from=backend /app/server /server
COPY --from=frontend /app/apps/portal/dist /static/portal
# Future: COPY --from=frontend /app/apps/hr-app/dist /static/hr
EXPOSE 8080
USER nonroot:nonroot
CMD ["/server"]
