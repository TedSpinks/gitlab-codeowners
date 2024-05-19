FROM alpine:latest

# https://wiki.alpinelinux.org/wiki/Setting_up_a_new_user
RUN addgroup -S gitlab && adduser -S gitlab -G gitlab -h /gitlab

COPY ./gitlab-codeowners /gitlab/

WORKDIR /gitlab

USER gitlab
