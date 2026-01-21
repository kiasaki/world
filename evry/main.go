package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed index.html sw.js app.webmanifest font.ttf icon.png
var assets embed.FS
var lock sync.Mutex
var cache = map[string][]byte{}

func main() {
	port := or(os.Getenv("PORT"), "8888")
	log.Println("starting on port", port)
	log.Fatalln(http.ListenAndServe("0.0.0.0:"+port,
		http.HandlerFunc(handler)))
}

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Now().Sub(start))
		if err := recover(); err != nil {
			log.Printf("panic: %s %s: %v", r.Method, r.URL.Path, err)
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("server error: %v", err)))
		}
	}()
	if r.Method == "POST" {
		user := strings.ReplaceAll(r.URL.Query().Get("user"), "=", "")
		body, err := ioutil.ReadAll(r.Body)
		check(err)
		r.Body.Close()
		lock.Lock()
		defer lock.Unlock()
		data := string(awsGet(user)) + string(body)
		awsPut(user, []byte(data))
		datums := strings.Split(data, "\n")
		checkpoint, err := strconv.Atoi(r.URL.Query().Get("checkpoint"))
		check(err)
		if checkpoint > len(datums) {
			checkpoint = len(datums)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(strings.Join(datums[checkpoint:], "\n")))
		log.Println("synced", user, checkpoint, len(datums), len(body))
		return
	}

	bs, err := fs.ReadFile(assets, r.URL.Path[1:])
	if err != nil {
		r.URL.Path = "/index.html"
		bs, _ = fs.ReadFile(assets, "index.html")
	}
	contentType := http.DetectContentType(bs)
	if ct, ok := map[string]string{
		"html":        "text/html",
		"webmanifest": "application/json",
		"js":          "application/javascript",
	}[strings.Split(r.URL.Path, ".")[1]]; ok {
		contentType = ct
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(bs)
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func awsGet(key string) []byte {
	if os.Getenv("AWS_ACCESS_KEY") == "" {
		bs, err := os.ReadFile("data/" + key)
		if err != nil {
			return []byte{}
		}
		return bs
	}
	if v, ok := cache[key]; ok {
		return v
	}
	target := fmt.Sprintf("https://%s.s3.amazonaws.com/%s",
		os.Getenv("AWS_BUCKET"), key)
	bs, err := aws("GET", target, nil, nil)
	if strings.Contains(fmt.Sprintf("%v", err), "NoSuchKey") {
		return nil
	}
	check(err)
	return bs
}

func awsPut(key string, value []byte) {
	if os.Getenv("AWS_ACCESS_KEY") == "" {
		check(os.MkdirAll(filepath.Dir("data/"+key), 0755))
		check(os.WriteFile("data/"+key, value, 0644))
		return
	}
	cache[key] = value
	_, err := aws(
		"PUT",
		fmt.Sprintf("https://%s.s3.amazonaws.com/%s",
			os.Getenv("AWS_BUCKET"), key),
		map[string]string{"content-type": or(http.DetectContentType(value), "application/octet-stream")},
		value,
	)
	check(err)
}

func aws(method, target string, headers map[string]string, body []byte) ([]byte, error) {
	var awsRegion = "us-east-1"
	var awsService = "s3"
	var awsAccessKey = os.Getenv("AWS_ACCESS_KEY")
	var awsSecretKey = os.Getenv("AWS_SECRET_KEY")
	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	check(err)

	// Headers
	payloadHash := hashAndEncode(body)
	now := time.Now().UTC()
	date := now.Format("20060102T150405Z")
	canonicalHeaders := ""
	headerNames := []string{"host", "x-amz-date", "x-amz-content-sha256"}
	headers2 := map[string]string{}
	headers2["host"] = req.URL.Host
	headers2["x-amz-date"] = date
	headers2["x-amz-content-sha256"] = payloadHash
	if headers != nil {
		for k, v := range headers {
			headerNames = append(headerNames, strings.ToLower(k))
			headers2[strings.ToLower(k)] = strings.TrimSpace(v)
		}
	}
	sort.Strings(headerNames)
	signedHeaders := strings.Join(headerNames, ";")
	for _, k := range headerNames {
		req.Header.Set(k, headers2[k])
		canonicalHeaders += fmt.Sprintf("%s:%s\n", k, headers2[k])
	}

	// Cannonical
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method, req.URL.Path, req.URL.Query().Encode(), canonicalHeaders, signedHeaders, payloadHash)
	canonicalRequestHash := hashAndEncode([]byte(canonicalRequest))
	algorithm := "AWS4-HMAC-SHA256"
	credentialDate := now.Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", credentialDate, awsRegion, awsService)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s", algorithm, date, credentialScope, canonicalRequestHash)

	// Sign

	kDate := sign([]byte("AWS4"+awsSecretKey), credentialDate)
	kRegion := sign(kDate, awsRegion)
	kService := sign(kRegion, awsService)
	signingKey := sign(kService, "aws4_request")
	signatureSHA := hmac.New(sha256.New, signingKey)
	signatureSHA.Write([]byte(stringToSign))
	signatureString := hex.EncodeToString(signatureSHA.Sum(nil))

	// Authorization Header
	authorizationHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", algorithm, awsAccessKey, credentialScope, signedHeaders, signatureString)
	req.Header.Set("Authorization", authorizationHeader)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	resBody, err := ioutil.ReadAll(res.Body)
	check(err)
	res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("aws error: %d: %s", res.StatusCode, string(resBody))
	}
	return resBody, nil
}

func hashAndEncode(s []byte) string {
	h := sha256.New()
	h.Write(s)
	return hex.EncodeToString(h.Sum(nil))
}

func sign(key []byte, message string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return h.Sum(nil)
}
