
<p align="center">
  <img height="300" src="https://onlypa.ws/share/2022/02/cuttle.png">
</p>

---
Cuttle is a heavily opinionated HTTP router in Go.
Its built on top of Echo so almost everything that's not new is most likely from echo.

## Usage
`http://localhost/test?q=hello+world&count=10`
```go
r := cuttle.New()
type testParam struct {
    ID    uint    `bind:"param"`
    Query string  `bind:"query" as:"q"`
    Count float64 `bind:"query"`
    Token string `bind:"header" as:"X-Security-Token,sensitive"`
}
r.GET("/test/:id", func(params testParam, ctx cuttle.Context) error {
    if !validateUser(params.Token) {
        return ctx.JSON(401, "unauthorized")	
    }   
    return ctx.JSON(200, map[string]interface{}{
        "request": params,
        "result":  fmt.Sprintf("Searched %v on %v", params.Query, params.ID),
    })
})

```
It can also be defined as an anonymous function.
```go
r := cuttle.New()
r.GET("/test/:id", func(params struct {
    ID    uint
    Query string
    Count float64
    Token string `bind:"header" as:"X-Security-Token,sensitive"`
}, ctx cuttle.Context) error {
    if !validateUser(params.Token) {
        return ctx.JSON(401, "unauthorized")	
    }   
    return ctx.JSON(200, map[string]interface{}{
        "request": params,
        "result":  fmt.Sprintf("Searched %v on %v", params.Query, params.ID),
    })
})

```

**More examples can be found in `router_test.go`**