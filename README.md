# phs

Default HTTP server and client prometheus instrumentation for go.
This library provides easy to use and non-intrusive methods to wrap HandlerFunc
and http Clients so that they automatically provide useful prometheus metrics.

## Server side metrics
All the server side metrics have the following labels:
  - **code** which is the http status code of the request
  - **handler** which is the name of the endpointk. It is set when wrapping a
  HandlerFunc. You should not use the local path of the URL for this, but choose
  a meaniingful name.
  - **method** the http method name like *get* or *post*

The default instrumentation provides metrics for
  - **http_server_request_total** is the number of requests received
  - **http_server_requests_inflight** is the number of requests currently being
    handled
  - **http_server_request_duration** is the prefix for the http latency buckets
    and percentile. The buckets and the percentiles can be defined. Defaults are
    provided.

The metrics are provided on a seperate port, using its own HttpServer
structure. This is good practice, because you don't want to block the server
providing the metrics in case the server serving the real application data is
overloaded or runs in a dead lock.

## Getting started

This project requires Go > 12.x. The main.go program provides an example of a
server which also performs as an http client. The server offers two endpoints,
*/expensive* and */cheap*. The */expensive* endpoint also calls back into the
same server's */cheap* endpoint to demo the client side wrapping.

Running it then should be as simple as:

```console
$ make build
$ ./bin/phs
```

## Testing

``make test``
