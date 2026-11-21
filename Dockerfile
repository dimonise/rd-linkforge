
FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /linkforged ./cmd/linkforged

FROM gcr.io/distroless/static:nonroot

COPY --from=build /linkforged /linkforged

EXPOSE 7070

ENTRYPOINT ["/linkforged"]
