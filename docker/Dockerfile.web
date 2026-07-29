ARG NGINX_BASE=docker.io/library/nginx:1.27-alpine

FROM ${NGINX_BASE}

ARG AUTHOR
ARG VERSION
LABEL maintainer=${AUTHOR} version=${VERSION}

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY dist /usr/share/nginx/html

EXPOSE 8087
