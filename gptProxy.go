package main

import (
	"io"
	"os"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tlsClient "github.com/bogdanfinn/tls-client"

	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
)

var (
	jar     = tlsClient.NewCookieJar()
	options = []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(360),
		tlsClient.WithClientProfile(tlsClient.Chrome_110),
		tlsClient.WithNotFollowRedirects(),
		tlsClient.WithCookieJar(jar), // create cookieJar instance and pass it as argument
	}
	client, _   = tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), options...)
	accessToken = os.Getenv("ACCESS_TOKEN")
	pUid        = os.Getenv("PUID")
)

func main() {
	if accessToken == "" && pUid == "" {
		println("Error: ACCESS_TOKEN and PUID are not set")
		return
	}
	// Automatically refresh the pUid cookie
	if accessToken != "" {
		go func() {
			url := "https://chat.openai.com/backend-api/models"
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Host", "chat.openai.com")
			req.Header.Set("origin", "https://chat.openai.com/chat")
			req.Header.Set("referer", "https://chat.openai.com/chat")
			req.Header.Set("sec-ch-ua", `Chromium";v="110", "Not A(Brand";v="24", "Brave";v="110`)
			req.Header.Set("sec-ch-ua-platform", "Linux")
			req.Header.Set("content-type", "application/json")
			req.Header.Set("content-type", "application/json")
			req.Header.Set("accept", "text/event-stream")
			req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36")
			// Set authorization header
			req.Header.Set("Authorization", "Bearer "+accessToken)
			// Initial pUid cookie
			req.AddCookie(
				&http.Cookie{
					Name:  "_puid",
					Value: pUid,
				},
			)
			resp, err := client.Do(req)
			if err != nil {
				// c.JSON(500, gin.H{"error": err.Error()})
				println(gin.H{"error": err.Error()})
				return
			}
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					// c.JSON(500, gin.H{"error": err.Error()})
					println(gin.H{"error": err.Error()})
					return
				}
			}(resp.Body)
			println("Got response: " + resp.Status)
			if resp.StatusCode != 200 {
				println("Error: " + resp.Status)
				// Print response body
				body, _ := io.ReadAll(resp.Body)
				println(string(body))
				return
			}
			// Get cookies from response
			cookies := resp.Cookies()
			// Find _puid cookie
			for _, cookie := range cookies {
				if cookie.Name == "_puid" {
					pUid = cookie.Value
					println("pUid: " + pUid)
					break
				}
			}
			// Sleep for 6 hour
			time.Sleep(6 * time.Hour)
			println("Error: Failed to refresh pUid cookie")
		}()
	}

	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "8080"
	}
	handler := gin.Default()
	handler.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	handler.Any("/api/*path", proxy)

	err := endless.ListenAndServe(os.Getenv("HOST")+":"+PORT, handler)
	if err != nil {
		return
	}
}

func proxy(c *gin.Context) {

	var url string
	var err error
	var requestMethod string
	var request *http.Request
	var response *http.Response

	url = "https://chat.openai.com/backend-api" + c.Param("path")
	requestMethod = c.Request.Method

	request, err = http.NewRequest(requestMethod, url, c.Request.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	request.Header.Set("Host", "chat.openai.com")
	request.Header.Set("Origin", "https://chat.openai.com/chat")
	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Keep-Alive", "timeout=360")
	request.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36")
	request.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	if c.Request.Header.Get("Puid") == "" {
		request.AddCookie(
			&http.Cookie{
				Name:  "_puid",
				Value: pUid,
			},
		)
	} else {
		request.AddCookie(
			&http.Cookie{
				Name:  "_puid",
				Value: c.Request.Header.Get("Puid"),
			},
		)
	}

	response, err = client.Do(request)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}(response.Body)
	c.Header("Content-Type", response.Header.Get("Content-Type"))
	// Get status code
	c.Status(response.StatusCode)
	c.Stream(func(w io.Writer) bool {
		// Write data to client
		_, _ = io.Copy(w, response.Body)
		return false
	})

}
