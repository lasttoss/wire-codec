# The image is the lab: one binary that can encode, decode and run the interchange gate, plus the
# declaration it is driven by. Nothing else, because nothing else is needed to show that the
# formats agree.
FROM golang:1.27 AS build

WORKDIR /src
# go.mod first, so that a source change does not re-download the module graph.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wirecodec ./cmd/wirecodec

# A library whose claim is "the decoder refuses what it does not understand" should not ship a
# shell for someone to paper over a failure with.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/wirecodec /wirecodec
COPY schema /schema
ENV WIRECODEC_SCHEMA=/schema/ops.yaml

ENTRYPOINT ["/wirecodec"]
