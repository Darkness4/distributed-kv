# ---
FROM --platform=$BUILDPLATFORM registry-1.docker.io/library/golang:1.23-alpine as builder

WORKDIR /build/
COPY go.mod go.su[m] ./
RUN go mod download

ARG TARGETOS TARGETARCH VERSION
COPY . /build/

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "-s -w -X main.version=${VERSION}" -o /build/dkv ./cmd/dkv/main.go
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "-s -w -X main.version=${VERSION}" -o /build/dkvctl ./cmd/dkvctl/main.go

# ---
FROM registry-1.docker.io/library/busybox:1.37.0

ARG TARGETOS TARGETARCH

ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-static-$TARGETARCH /tini
RUN chmod +x /tini

RUN mkdir /app
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app

COPY --from=builder /build/dkv /build/dkvctl /app/

RUN chown -R app:app .
USER app

EXPOSE 3000

ENTRYPOINT ["/tini", "--", "/app/dkv"]
