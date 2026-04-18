# Stage 1: Build frontend
FROM node:20-slim AS frontend
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY tsconfig.base.json ./
COPY packages/ packages/
COPY apps/ apps/
RUN pnpm install --frozen-lockfile && pnpm -r build

# Stage 2: Build backend
FROM golang:1.25 AS backend
WORKDIR /app
COPY backend/ ./
RUN go mod download && CGO_ENABLED=0 go build -o server ./cmd/server

# Stage 3: Production image
FROM gcr.io/distroless/static-debian12
COPY --from=backend /app/server /server
COPY --from=frontend /app/apps/portal/dist /static
COPY --from=frontend /app/apps/budget/dist /static/apps/budget
COPY --from=frontend /app/apps/compliance/dist /static/apps/compliance
COPY --from=frontend /app/apps/coperture/dist /static/apps/coperture
COPY --from=frontend /app/apps/energia-dc/dist /static/apps/energia-dc
COPY --from=frontend /app/apps/kit-products/dist /static/apps/kit-products
COPY --from=frontend /app/apps/listini-e-sconti/dist /static/apps/listini-e-sconti
COPY --from=frontend /app/apps/panoramica-cliente/dist /static/apps/panoramica-cliente
COPY --from=frontend /app/apps/quotes/dist /static/apps/quotes
COPY --from=frontend /app/apps/richieste-fattibilita/dist /static/apps/richieste-fattibilita
COPY --from=frontend /app/apps/rdf-backend/dist /static/apps/rdf-backend
COPY --from=frontend /app/apps/reports/dist /static/apps/reports
COPY --from=frontend /app/apps/simulatori-vendita/dist /static/apps/simulatori-vendita
EXPOSE 8080
USER nonroot:nonroot
CMD ["/server"]
