FROM registry.opensuse.org/opensuse/bci/bci-init:latest
WORKDIR /app
ARG TARGETARCH

LABEL org.opencontainers.image.description="Athanor: backup your Materia components"
LABEL org.opencontainers.image.licenses=GPLv3

WORKDIR /app
RUN mkdir -p /lib64
RUN mkdir -p /root/.ssh && \
	chmod 0700 /root/.ssh && \
	touch /root/.ssh/known_hosts

RUN zypper in -y podman && zypper clean

COPY ./bin/athanor-${TARGETARCH} /app/athanor

ENTRYPOINT ["/app/athanor"]
