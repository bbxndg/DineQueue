# Local development target; release images/pipeline are Lab 5 work.
FROM golang:1.26.10-bookworm AS development
WORKDIR /app
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN useradd --create-home --uid 10001 app && chown -R app:app /app
USER app
ENV GOTOOLCHAIN=local
EXPOSE 8080
CMD ["go", "run", "-buildvcs=false", "./cmd/api"]

# Compile API and database tools; migrations are embedded by the db package.
FROM development AS build
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags timetzdata -o /app/bin/api ./cmd/api \
    && CGO_ENABLED=0 go build -buildvcs=false -trimpath -tags timetzdata -o /app/bin/db ./cmd/db

# Static binaries include timezone data; retain trust roots for verified DB TLS.
FROM scratch AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 10001:10001
WORKDIR /app

FROM runtime AS db-tools
COPY --from=build /app/bin/db /app/db
ENTRYPOINT ["/app/db"]
CMD ["migrate"]

FROM runtime AS production
COPY --from=build /app/bin/api /app/api
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
