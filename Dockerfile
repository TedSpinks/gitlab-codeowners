FROM alpine:latest

# https://wiki.alpinelinux.org/wiki/Setting_up_a_new_user
# RUN addgroup -S gitlab && adduser -S gitlab -G gitlab -h /gitlab

WORKDIR /gitlab

COPY ./gitlab-codeowners /gitlab/

RUN chown -R 1001:0 /gitlab && chmod -R g=u /gitlab

USER 1001
