ARG BASE_IMAGE=codexppp-backend
FROM ${BASE_IMAGE}

USER root
COPY --chmod=0755 codexppp-backend /usr/local/bin/codexppp-backend
USER codexppp
