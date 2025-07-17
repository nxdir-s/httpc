# httpc

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
resp, err := client.Get(ctx, "/resource", nil, &response)
if err != nil {
    // handle error
}
defer resp.Body.Close()
```

POST Request

```go
var response Response
resp, err := client.Post(ctx, "/resource", bytes.NewReader(body), nil, &response)
if err != nil {
    // handle error
}
defer resp.Body.Close()
```

PUT Request

```go
var response Response
resp, err := client.Put(ctx, "/resource", bytes.NewReader(body), nil, &response)
if err != nil {
    // handle error
}
defer resp.Body.Close()
```

DELETE Request

```go
var response Response
resp, err := client.Delete(ctx, "/resource", bytes.NewReader(body), nil, &response)
if err != nil {
    // handle error
}
defer resp.Body.Close()
```

PATCH Request

```go
var response Response
resp, err := client.Patch(ctx, "/resource", bytes.NewReader(body), nil, &response)
if err != nil {
    // handle error
}
defer resp.Body.Close()
```
