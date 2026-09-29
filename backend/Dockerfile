FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/prezzi-benzina ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/prezzi-benzina /prezzi-benzina
EXPOSE 8080
ENTRYPOINT ["/prezzi-benzina"]
