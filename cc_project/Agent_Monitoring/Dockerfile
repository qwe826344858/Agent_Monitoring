FROM scratch

WORKDIR /app

COPY agent-monitor .
COPY static/ ./static/

EXPOSE 8080

CMD ["./agent-monitor", "-port", "8080"]
