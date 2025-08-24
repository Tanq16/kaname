FROM golang:alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/kaname -ldflags "-s -w" main.go

# ----

FROM ubuntu:jammy

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install --no-install-recommends -y python3 python3-venv python3-requests python3-yaml \
    ca-certificates zip unzip git ffmpeg curl wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir testingground && cd testingground && \
    a=$(curl -s https://api.github.com/repos/tanq16/danzo/releases/latest | grep -E "browser_download_url.*" | grep linux-amd64 | cut -d '"' -f4) && \
    wget "$a" -O danzo.zip && \
    unzip danzo.zip && mv danzo /usr/local/bin/danzo && rm * && \
    danzo ghrelease tanq16/raikiri && \
    mv raikiri-* raikiri && chmod +x raikiri && \
    mv raikiri /usr/local/bin/raikiri && \
    danzo ghrelease tanq16/ai-context && \
    unzip *.zip && rm LICENSE README.md *.zip && \
    mv ai-context /usr/local/bin/ai-context && \
    danzo ghrelease tanq16/anbu && \
    unzip *.zip && rm LICENSE README.md *.zip && \
    mv anbu /usr/local/bin/anbu && \
    cd .. && rmdir testingground
WORKDIR /app
RUN mkdir -p /app/scripts
COPY --from=builder /bin/kaname /app/kaname

EXPOSE 8080
CMD ["./kaname"]
