FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH=amd64
COPY bin/linux-${TARGETARCH}/mayday /mayday
USER 65532:65532
EXPOSE 8080 8081
ENTRYPOINT ["/mayday"]
CMD ["monitor"]
