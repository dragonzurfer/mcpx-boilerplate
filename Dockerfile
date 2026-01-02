FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app .

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=build /out/app /app/app
COPY --from=build /src/web /app/web
COPY --from=build /src/config /app/config


EXPOSE 8080

CMD ["/app/app"]
