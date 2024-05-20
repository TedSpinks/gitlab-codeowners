FROM alpine:latest

# https://wiki.alpinelinux.org/wiki/Setting_up_a_new_user
RUN addgroup -S gitlab && adduser -S gitlab -G gitlab -h /gitlab

USER gitlab

WORKDIR /gitlab

COPY ./gitlab-codeowners /gitlab/
