package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
var lock sync.RWMutex
var cache = map[string][]byte{}

type J map[string]interface{}

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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			w.Write(jsonEncode(J{"error": fmt.Sprintf("%v", err)}))
		}
	}()
	if r.Method == "POST" {
		rpc(w, r)
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

func rpc(w http.ResponseWriter, r *http.Request) {
	code := 200
	ret := J{}
	b := jsonDecode(ioMustRead(r.Body))
	if js(b, "method") == "userGet" {
		username := js(b, "username")
		id := string(s3Get("n|" + username))
		ret = jsonDecode(s3Get("u|" + id))
		delete(ret, "secret")
	} else {
		id := js(b, "id")
		user := jsonDecode(s3Get("u|" + id))
		if len(user) > 0 && js(user, "secret") != js(b, "secret") {
			panic("unauthorized")
		}
		switch js(b, "method") {
		case "userPut":
			username := js(b, "username")
			if len(s3Get("n|"+username)) == 0 {
				s3Put("n|"+username, []byte(id))
			}
			user["id"] = id
			user["salt"] = js(b, "salt")
			user["secret"] = js(b, "secret")
			user["master"] = js(b, "master")
			s3Put("u|"+id, jsonEncode(user))
		case "datumGet":
			checkpoint := strings.Split(or(js(b, "checkpoint"), "0.0"), ".")
			chunk, _ := strconv.ParseInt(checkpoint[0], 10, 64)
			index, _ := strconv.ParseInt(checkpoint[1], 10, 64)
			k := fmt.Sprintf("d|%s|%d", id, chunk)
			datums := jsonDecode([]byte(or(string(s3Get(k)), `{"datums":[]}`)))["datums"].([]interface{})
			c := fmt.Sprintf("%d.%d", chunk, len(datums))
			if chunk < ji(user, "chunk") {
				c = fmt.Sprintf("%d.0", chunk+1)
			}
			ret = J{"datums": datums[index:], "checkpoint": c}
		case "datumPut":
			user := jsonDecode(s3Get("u|" + id))
			chunk := ji(user, "chunk")
			k := fmt.Sprintf("d|%s|%d", id, chunk)
			d := jsonDecode([]byte(or(string(s3Get(k)), `{"datums":[]}`)))
			datums := d["datums"].([]interface{})
			datums = append(datums, b["datums"].([]interface{})...)
			data := jsonEncode(J{"datums": datums})
			s3Put(k, data)
			if len(data) >= 1_048_576 { // 1mb
				chunk++
				datums = []interface{}{}
				user["chunk"] = chunk
				s3Put("u|"+id, jsonEncode(user))
			}
		case "blobGet":
			ret = J{"data": s3Get("b|" + id + "|" + js(b, "blob"))}
		case "blobPut":
			data := []byte(js(b, "data"))
			bid := "b|" + id + "|" + hashAndEncode(data)
			if len(s3Get(bid)) == 0 {
				s3Put(bid, data)
			}
		default:
			code = 500
			ret = J{"error": "unknown method"}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(jsonEncode(ret))
}

func js(j J, k string) string {
	if v, ok := j[k].(string); ok {
		return v
	}
	return ""
}

func ji(j J, k string) int64 {
	if v, ok := j[k].(float64); ok {
		return int64(v)
	}
	return 0
}

func ioMustRead(r io.ReadCloser) []byte {
	b, err := io.ReadAll(r)
	r.Close()
	check(err)
	return b
}

func jsonDecode(b []byte) (v J) {
	if len(b) == 0 {
		return J{}
	}
	check(json.Unmarshal(b, &v))
	return v
}

func jsonEncode(v interface{}) []byte {
	b, err := json.Marshal(v)
	check(err)
	return b
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

func s3Get(key string) []byte {
	key = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, "/", "_"), "+", "-"), "|", "/")
	lock.RLock()
	defer lock.RUnlock()
	if os.Getenv("AWS_ACCESS_KEY") == "" {
		bs, err := os.ReadFile("data/" + key)
		if err != nil {
			return nil
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

func s3Put(key string, value []byte) {
	key = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(key, "/", "_"), "+", "-"), "|", "/")
	lock.Lock()
	defer lock.Unlock()
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
