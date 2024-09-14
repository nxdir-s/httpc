# HttpC

This repository contains an `net/http` client wrapper that handles authentication and can be configured with rate limiting, retries and Open Telemetry instrumentation.

Example client setup

```go
cfg := &httpc.Config{
    BaseUrl:        "google.com",
    Timeout:        10,
    RetryEnabled:   true,
    TlsConfig: &tls.Config{
        InsecureSkipVerify: false,
        MinVersion:         tls.VersionTLS12,
    },
}

client, err := httpc.NewClient(ctx, cfg)
if err != nil {
    // handle error
}
```

GET Request

```go
var response Response
_, err := client.Get(ctx, "/resource", nil, &response)
if err != nil {
    // handle error
}
```

POST Request

```go
var response Response
_, err := client.Post(ctx, "/resource", bytes.NewReader(body), nil, &response)
if err != nil {
    // handle error
}
```
