rp
--

A toy reverse proxy that does not use `net/httputil`

Do not use anywhere in production.

Backends are HTTP only.

## Plan

Iterates over simple features

- simple http/1.1
- reverse lb
- cache
- x forwarded for

## TODO

- xforwarded for
- sticky (crc ip)
- http2
- websocket
- backend failure detection and eviction
- dynamic discovery of the backend list 