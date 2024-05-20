FROM alpine:latest

WORKDIR /gitlab

COPY ./gitlab-codeowners /gitlab/

RUN chown -R 1001:0 /gitlab && chmod -R g=u /gitlab

USER 1001
