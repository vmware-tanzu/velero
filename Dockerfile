# Build style checker
FROM golang:1.9.4 as vale
RUN go get github.com/ValeLint/vale

# Build link checker
FROM ruby:2.5.0 as htmlproofer
RUN gem install html-proofer
ENTRYPOINT which htmlproofer

# Build site
FROM jekyll/jekyll
ENV EXEC_DIR /srv/jekyll
COPY --from=vale /go/bin/vale .
COPY --from=htmlproofer /usr/local/bundle/bin/htmlproofer .
ADD test-docs.sh .

# Execute tests
WORKDIR /project
ENTRYPOINT $EXEC_DIR/test-docs.sh