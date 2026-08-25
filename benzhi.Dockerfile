FROM golang:1.26 AS build
WORKDIR /src
ENV GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal
RUN go build -mod=vendor -o /out/tideshield ./cmd/tideshield

FROM golang:1.26
WORKDIR /app
ENV GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
COPY --from=build /out/tideshield /app/tideshield
COPY --from=build /src/go.mod /src/go.sum /app/
COPY --from=build /src/vendor /app/vendor
COPY --from=build /src/cmd /app/cmd
COPY --from=build /src/internal /app/internal
COPY web /app/web
RUN mkdir -p /app/data
EXPOSE 21226
CMD ["/app/tideshield", "-addr", "0.0.0.0:21226", "-data", "/app/data", "-web", "/app/web"]
