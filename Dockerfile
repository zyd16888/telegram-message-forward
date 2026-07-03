FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --chmod=755 dist/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
