FROM golang:alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/kaname -ldflags "-s -w" main.go

# ----

FROM ubuntu:jammy

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y python3 python3-venv ca-certificates git && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /app
RUN mkdir -p /app/scripts
COPY --from=builder /bin/kaname /app/kaname

EXPOSE 8080
CMD ["./kaname"]
