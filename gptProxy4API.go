package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	baseUrl      = "https://api.openai.com"
	OpenAIApiKey string
)

func main() {
	if OpenAIApiKey = os.Getenv("OPENAI_API_KEY"); len(OpenAIApiKey) > 1 {
		log.Println("Configured OPENAI_API_KEY")
	} else {
		log.Println("Not configured OPENAI_API_KEY")
	}
	router := http.NewServeMux()
	router.HandleFunc("/", HandleProxy)
	fmt.Println("API proxy server is listening on port 9000")
	if err := http.ListenAndServe(":9000", router); err != nil {
		panic(err)
	}
}

func HandleProxy(w http.ResponseWriter, r *http.Request) {
	client := http.DefaultClient
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	client.Transport = tr

	req, err := http.NewRequest(r.Method, baseUrl+r.URL.Path, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header
	if len(OpenAIApiKey) > 1 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", OpenAIApiKey))
	}

	req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("Connection", "keep-alive")

	rsp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}(rsp.Body)

	for name, values := range rsp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	head := map[string]string{
		"Cache-Control":                    "no-store",
		"access-control-allow-origin":      "*",
		"access-control-allow-credentials": "true",
		"Transfer-Encoding":                "chunked",
		"Connection":                       "keep-alive",
	}
	for k, v := range head {
		if _, ok := rsp.Header[k]; !ok {
			w.Header().Set(k, v)
		}
	}

	rsp.Header.Del("content-security-policy")
	rsp.Header.Del("content-security-policy-report-only")
	rsp.Header.Del("clear-site-data")
	w.Header().Set("Accept-Encoding", "gzip")

	w.WriteHeader(rsp.StatusCode)

	scanner := bufio.NewScanner(rsp.Body)
	for scanner.Scan() {
		_, _ = w.Write([]byte(strconv.Itoa(len(scanner.Text())) + "\r\n"))
		_, _ = w.Write(scanner.Bytes())
		_, _ = w.Write([]byte("\r\n"))
		w.(http.Flusher).Flush()

	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to read response: %v", err)
	}

	/*w.(http.Flusher).Flush()
	if _, err := io.Copy(w, rsp.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}*/

}
