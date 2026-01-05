FROM alpine

ENV GAPS_HISTORY_GRADES_FILE="/history/grades.json"

ENTRYPOINT ["/usr/local/bin/gaps-cli"]
COPY gaps-cli /usr/local/bin/gaps-cli

CMD ["--help"]
