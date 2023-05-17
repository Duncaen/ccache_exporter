FROM golang:1.20 AS build
ADD . /app
WORKDIR /app
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o ccache_exporter
FROM scratch
COPY --from=build /app/ccache_exporter /ccache_exporter
EXPOSE 9058
ENTRYPOINT ["/ccache_exporter"]
