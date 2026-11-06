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
