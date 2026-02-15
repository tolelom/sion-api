FROM alpine:latest
WORKDIR /app
COPY server .
EXPOSE 8001
CMD ["./server"]