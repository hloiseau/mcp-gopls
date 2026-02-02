FROM golang:1.25.4-bookworm

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates git \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /usr/local/bin/mcp-gopls ./cmd/mcp-gopls \
  && go install golang.org/x/tools/gopls@latest

ENV MCP_GOPLS_GOPLS_PATH=/go/bin/gopls

WORKDIR /workspace
ENTRYPOINT ["mcp-gopls"]
CMD ["--workspace", "/workspace"]
