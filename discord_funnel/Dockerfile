# --- Build stage --------------------------------------------------------
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod ./
# No go.sum is committed; resolve + verify + lock dependencies at build time
# (the Supervisor build has network access; this keeps the repo dependency-free).
RUN go mod tidy

COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /discord-funnel .

# --- Runtime stage -------------------------------------------------------
FROM alpine:3.19

RUN apk add --no-cache jq bash ca-certificates

COPY --from=builder /discord-funnel /usr/bin/discord-funnel
COPY run.sh /run.sh
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
