package main

import (
	"io/ioutil"
	"napnap"
	"net/http"
	"strings"
)

var (
	_apis []Api
)

func init() {
	api := Api{
		Name:             "Test",
		RequestHost:      "localhost",
		RequestPath:      "/",
		StripRequestPath: true,
	}

	_apis = append(_apis, api)
}

func main() {
	nap := napnap.New()

	nap.UseFunc(func(c *napnap.Context, next napnap.HandlerFunc) {
		api := Api{
			Name:             "Test",
			RequestHost:      "localhost",
			RequestPath:      "/api",
			StripRequestPath: true,
			TargetUrl:        "http://localhost",
		}

		// if the request url doesn't match, we will by pass it
		requestPath := c.Request.URL.Path
		if !strings.HasPrefix(requestPath, api.RequestPath) {
			return
		}

		// get information
		//ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		//println("ip:" + ip)

		// exchange url
		var url string
		if api.StripRequestPath {
			newPath := strings.TrimPrefix(requestPath, api.RequestPath)
			url = api.TargetUrl + newPath
		} else {
			url = api.TargetUrl + requestPath
		}

		rawQuery := c.Request.URL.RawQuery
		if len(rawQuery) > 0 {
			url += "?" + rawQuery
		}

		//fmt.Println("URL:>", url)

		method := c.Request.Method
		req, err := http.NewRequest(method, url, c.Request.Body)

		// copy the request header
		copyHeader(req.Header, c.Request.Header)

		// send to target
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// copy the response header
		copyHeader(c.Writer.Header(), resp.Header)
		//c.Writer.Header().Set("X-Forwarded-For", "127.0.2.32")

		// write body
		body, _ := ioutil.ReadAll(resp.Body)
		c.Writer.Write(body)
	})

	nap.Run(":8080")
}
