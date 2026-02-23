package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func runObject(args []string) {
	if len(args) == 0 {
		fmt.Println(`Usage: vaults3-cli object <subcommand>

Subcommands:
  ls <bucket> [--prefix=<prefix>] [--max-keys=<n>]   List objects
  put <bucket> <key> <file>                           Upload object
  get <bucket> <key> <file>                           Download object
  rm <bucket> <key>                                   Delete object
  cp <src-bucket/key> <dst-bucket/key>                Copy object
  presign <bucket> <key> [--expires=3600]             Generate presigned GET URL`)
		os.Exit(1)
	}

	requireCreds()

	switch args[0] {
	case "ls", "list":
		objectList(args[1:])
	case "put", "upload":
		objectPut(args[1:])
	case "get", "download":
		objectGet(args[1:])
	case "rm", "delete":
		objectDelete(args[1:])
	case "cp", "copy":
		objectCopy(args[1:])
	case "presign":
		objectPresign(args[1:])
	default:
		fatal("unknown object subcommand: " + args[0])
	}
}

func objectList(args []string) {
	if len(args) < 1 {
		fatal("object ls requires a bucket name")
	}
	bucket := args[0]
	prefix := ""
	maxKeys := 1000

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--prefix=") {
			prefix = strings.TrimPrefix(arg, "--prefix=")
		} else if strings.HasPrefix(arg, "--max-keys=") {
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-keys="))
			if err == nil {
				maxKeys = n
			}
		}
	}

	path := fmt.Sprintf("/%s?list-type=2&max-keys=%d", bucket, maxKeys)
	if prefix != "" {
		path += "&prefix=" + url.QueryEscape(prefix)
	}

	resp, err := s3Request("GET", path, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		XMLName  xml.Name `xml:"ListBucketResult"`
		Contents []struct {
			Key          string `xml:"Key"`
			Size         int64  `xml:"Size"`
			LastModified string `xml:"LastModified"`
			ETag         string `xml:"ETag"`
		} `xml:"Contents"`
		KeyCount int `xml:"KeyCount"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("parse response: " + err.Error())
	}

	if len(result.Contents) == 0 {
		fmt.Println("No objects found.")
		return
	}

	headers := []string{"KEY", "SIZE", "LAST MODIFIED", "ETAG"}
	var rows [][]string
	for _, obj := range result.Contents {
		t, _ := time.Parse(time.RFC3339Nano, obj.LastModified)
		rows = append(rows, []string{
			obj.Key,
			formatSize(obj.Size),
			t.Format("2006-01-02 15:04:05"),
			strings.Trim(obj.ETag, "\""),
		})
	}
	printTable(headers, rows)
	fmt.Printf("\n%d object(s)\n", len(result.Contents))
}

func objectPut(args []string) {
	if len(args) < 3 {
		fatal("object put requires: <bucket> <key> <file>")
	}
	bucket, key, filePath := args[0], args[1], args[2]

	f, err := os.Open(filePath)
	if err != nil {
		fatal(err.Error())
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		fatal(err.Error())
	}

	data, err := io.ReadAll(f)
	if err != nil {
		fatal(err.Error())
	}

	path := fmt.Sprintf("/%s/%s", bucket, key)
	resp, err := s3Request("PUT", path, bytes.NewReader(data))
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 204 {
		fmt.Printf("Uploaded '%s' to %s/%s (%s)\n", filePath, bucket, key, formatSize(stat.Size()))
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func objectGet(args []string) {
	if len(args) < 3 {
		fatal("object get requires: <bucket> <key> <file>")
	}
	bucket, key, filePath := args[0], args[1], args[2]

	path := fmt.Sprintf("/%s/%s", bucket, key)
	resp, err := s3Request("GET", path, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	out, err := os.Create(filePath)
	if err != nil {
		fatal(err.Error())
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf("Downloaded %s/%s to '%s' (%s)\n", bucket, key, filePath, formatSize(n))
}

func objectDelete(args []string) {
	if len(args) < 2 {
		fatal("object rm requires: <bucket> <key>")
	}
	bucket, key := args[0], args[1]

	path := fmt.Sprintf("/%s/%s", bucket, key)
	resp, err := s3Request("DELETE", path, nil)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		fmt.Printf("Deleted %s/%s\n", bucket, key)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func objectCopy(args []string) {
	if len(args) < 2 {
		fatal("object cp requires: <src-bucket/key> <dst-bucket/key>")
	}
	srcParts := strings.SplitN(args[0], "/", 2)
	dstParts := strings.SplitN(args[1], "/", 2)

	if len(srcParts) != 2 || len(dstParts) != 2 {
		fatal("source and destination must be in format: bucket/key")
	}

	path := fmt.Sprintf("/%s/%s", dstParts[0], dstParts[1])
	url := strings.TrimRight(endpoint, "/") + path

	req, err := newHTTPRequest("PUT", url, nil)
	if err != nil {
		fatal(err.Error())
	}
	req.Header.Set("X-Amz-Copy-Source", fmt.Sprintf("/%s/%s", srcParts[0], srcParts[1]))
	signV4(req, accessKey, secretKey, region)

	resp, err := httpClient().Do(req)
	if err != nil {
		fatal(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Printf("Copied %s to %s\n", args[0], args[1])
	} else {
		body, _ := io.ReadAll(resp.Body)
		fatal(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
}

func objectPresign(args []string) {
	if len(args) < 2 {
		fatal("object presign requires: <bucket> <key> [--expires=3600]")
	}
	bucket, key := args[0], args[1]
	expires := 3600

	for _, arg := range args[2:] {
		if strings.HasPrefix(arg, "--expires=") {
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--expires="))
			if err == nil {
				expires = n
			}
		}
	}

	// Generate presigned URL locally
	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	u, _ := url.Parse(endpoint)
	host := u.Host

	credential := fmt.Sprintf("%s/%s/%s/s3/aws4_request", accessKey, dateStr, region)

	params := url.Values{}
	params.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	params.Set("X-Amz-Credential", credential)
	params.Set("X-Amz-Date", amzDate)
	params.Set("X-Amz-Expires", strconv.Itoa(expires))
	params.Set("X-Amz-SignedHeaders", "host")

	canonicalURI := fmt.Sprintf("/%s/%s", bucket, key)
	canonicalQueryString := params.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", host)
	signedHeaders := "host"

	canonicalRequest := fmt.Sprintf("GET\n%s\n%s\n%s\n%s\nUNSIGNED-PAYLOAD",
		canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders)

	hash := sha256.Sum256([]byte(canonicalRequest))
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(hash[:]))

	kDate := hmacSign([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := hmacSign(kDate, []byte(region))
	kService := hmacSign(kRegion, []byte("s3"))
	kSigning := hmacSign(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSign(kSigning, []byte(stringToSign)))

	params.Set("X-Amz-Signature", signature)

	fmt.Printf("%s%s?%s\n", endpoint, canonicalURI, params.Encode())
}

func newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}
