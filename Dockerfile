# Warmline — one static binary, one SQLite file, no runtime deps.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /warmline ./cmd/warmline

FROM scratch
COPY --from=build /warmline /warmline
# SQLite file lives on a mounted volume; scratch has no shell/TLS roots,
# outbound relay uses STARTTLS with system roots — mount /etc/ssl/certs
# read-only if your relay needs a custom CA.
VOLUME ["/data"]
ENV WARMLINE_DB=/data/warmline.db
ENTRYPOINT ["/warmline"]
CMD ["serve"]
