package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"

	"io"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	HttpClient HTTPClient
)

func init() {
	HttpClient = &http.Client{Transport: &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
	}}
}

type UploadOption struct {
	Method    string
	UploadUrl string
	Filename  string
	MimeType  string
	PairMap   map[string]string
}

func Head(url string) (reqLen int, respLen int, err error) {
	request, err := http.NewRequest("HEAD", url, nil)
	reqBuf := GetBuffer()
	defer PutBuffer(reqBuf)
	request.Write(reqBuf)
	reqLen = reqBuf.Len()

	resp, err := HttpClient.Do(request)
	if err != nil {
		return
	}
	defer CloseResponse(resp)

	respBuf := GetBuffer()
	defer PutBuffer(respBuf)
	resp.Header.Write(respBuf)
	respLen = respBuf.Len()

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusAccepted, http.StatusOK, http.StatusNoContent:
		return reqLen, respLen, err
	}

	return reqLen, respLen, fmt.Errorf("delete path:%s err, httpCode:%v", url, resp.StatusCode)
}

func Delete(url string) (reqLen int, respLen int, err error) {
	request, err := http.NewRequest("DELETE", url, nil)
	reqBuf := GetBuffer()
	defer PutBuffer(reqBuf)
	request.Write(reqBuf)
	reqLen = reqBuf.Len()

	resp, err := HttpClient.Do(request)
	if err != nil {
		return
	}
	defer CloseResponse(resp)

	respBuf := GetBuffer()
	defer PutBuffer(respBuf)
	resp.Header.Write(respBuf)
	respLen = respBuf.Len()

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusAccepted, http.StatusOK, http.StatusNoContent:
		return reqLen, respLen, err
	}
	body, err := io.ReadAll(resp.Body)

	return reqLen, respLen, fmt.Errorf("delete path:%s err, httpCode:%v,body:%s", url, resp.StatusCode, string(body))
}

func Get(url string) (reqLen int, respLen int, needRetry bool, err error) {

	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("Accept-Encoding", "gzip")
	reqBuf := GetBuffer()
	defer PutBuffer(reqBuf)
	request.Write(reqBuf)
	reqLen = reqBuf.Len()

	response, err := HttpClient.Do(request)
	if err != nil {
		return reqLen, respLen, true, err
	}
	defer CloseResponse(response)

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		defer reader.Close()
	default:
		reader = response.Body
	}
	b, err := io.ReadAll(reader)

	respBuf := GetBuffer()
	defer PutBuffer(respBuf)
	err2 := response.Header.Write(respBuf)

	if response.StatusCode >= 400 {
		retryable := response.StatusCode >= 500
		return reqLen, respLen, retryable, fmt.Errorf("%s: %s", url, response.Status)
	}
	if err != nil || err2 != nil {
		return reqLen, respLen, false, err
	}
	respLen = len(b) + respBuf.Len()
	return
}

func upload_body(fillBufferFunction func(w io.Writer) error, option *UploadOption) (reqLen int, respLen int, err error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	req, postErr := http.NewRequest(option.Method, option.UploadUrl, bytes.NewReader(buf.Bytes()))
	if postErr != nil {
		err = fmt.Errorf("create upload request %s: %v", option.UploadUrl, postErr)
		return
	}
	req.Header.Set("Content-Type", option.MimeType)
	for k, v := range option.PairMap {
		req.Header.Set(k, v)
	}
	if err = fillBufferFunction(buf); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return
	}

	headBuf := GetBuffer()
	defer PutBuffer(headBuf)
	req.Header.Write(headBuf)
	reqLen = buf.Len() + headBuf.Len()

	// print("+")
	resp, post_err := HttpClient.Do(req)
	if post_err != nil {
		if strings.Contains(post_err.Error(), "connection reset by peer") ||
			strings.Contains(post_err.Error(), "use of closed network connection") {
			resp, post_err = HttpClient.Do(req)
		}
	}
	if post_err != nil {
		err = fmt.Errorf("post addr:%s, err: %v", option.UploadUrl, post_err)
		return
	}
	defer CloseResponse(resp)

	respBuf := GetBuffer()
	defer PutBuffer(respBuf)
	resp.Write(respBuf)
	respLen = respBuf.Len()

	if resp.StatusCode < 400 {
		return
	}

	resp_body, ra_err := io.ReadAll(resp.Body)
	if ra_err != nil {
		err = fmt.Errorf("read response body %v: %v", option.UploadUrl, ra_err)
		return
	}
	err = fmt.Errorf("read response body %v: %v", option.UploadUrl, string(resp_body))
	return
}

func upload_content(fillBufferFunction func(w io.Writer) error, option *UploadOption) (reqLen int, respLen int, err error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	body_writer := multipart.NewWriter(buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, option.Filename))
	h.Set("Idempotency-Key", option.UploadUrl)
	if option.MimeType == "" {
		option.MimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(option.Filename)))
	}
	if option.MimeType != "" {
		h.Set("Content-Type", option.MimeType)
	}

	file_writer, cp_err := body_writer.CreatePart(h)
	if cp_err != nil {
		log.Printf("error creating form file %s\n", cp_err.Error())
		return reqLen, respLen, cp_err
	}
	if err := fillBufferFunction(file_writer); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return reqLen, respLen, err
	}
	content_type := body_writer.FormDataContentType()
	if err = body_writer.Close(); err != nil {
		log.Printf("error closing body %s\n", err.Error())
		return reqLen, respLen, err
	}
	req, postErr := http.NewRequest("POST", option.UploadUrl, bytes.NewReader(buf.Bytes()))
	if postErr != nil {
		log.Printf("create upload request %s: %v\n", postErr)
		err = fmt.Errorf("create upload request %s: %v", option.UploadUrl, postErr)
		return
	}
	req.Header.Set("Content-Type", content_type)
	for k, v := range option.PairMap {
		req.Header.Set(k, v)
	}

	headBuf := GetBuffer()
	defer PutBuffer(headBuf)
	req.Header.Write(headBuf)

	reqLen = buf.Len() + headBuf.Len()

	// print("+")
	resp, post_err := HttpClient.Do(req)
	if post_err != nil {
		if strings.Contains(post_err.Error(), "connection reset by peer") ||
			strings.Contains(post_err.Error(), "use of closed network connection") {
			resp, post_err = HttpClient.Do(req)
		}
	}
	if post_err != nil {
		err = fmt.Errorf("upload %s %d bytes to  %v", option.Filename, option.UploadUrl, post_err)
		return
	}
	defer CloseResponse(resp)

	if resp.StatusCode < 400 {
		return
	}

	resp_body, ra_err := io.ReadAll(resp.Body)
	if ra_err != nil {
		err = fmt.Errorf("read response body %v: %v", option.UploadUrl, ra_err)
		return
	}
	err = fmt.Errorf("read response body %v: %v", option.UploadUrl, string(resp_body))
	return
}

func Upload(reader io.Reader, option *UploadOption) (reqLen int, respLen int, err error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		err = fmt.Errorf("read input: %v", err)
		return
	}
	for i := 0; i < 3; i++ {
		if option.MimeType == "multipart/form-data" {
			reqLen, respLen, err = upload_content(func(w io.Writer) (err error) {
				_, err = w.Write(data)
				return
			}, option)
		} else {
			reqLen, respLen, err = upload_body(func(w io.Writer) (err error) {
				_, err = w.Write(data)
				return
			}, option)
		}
		if err == nil {
			return
		} else {
			log.Printf("uploading to %s: %v", option.UploadUrl, err)
		}
		time.Sleep(time.Millisecond * time.Duration(237*(i+1)))
	}
	return
}

func CloseResponse(resp *http.Response) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

var (
	sharedBytes []byte
)

type FakeReader struct {
	id     uint64 // an id number
	size   int64  // max bytes
	random *rand.Rand
}

func (l *FakeReader) Read(p []byte) (n int, err error) {
	if l.size <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.size {
		n = int(l.size)
	} else {
		n = len(p)
	}
	if n >= 8 {
		for i := 0; i < 8; i++ {
			p[i] = byte(l.id >> uint(i*8))
		}
		l.random.Read(p[8:])
	}
	l.size -= int64(n)
	return
}

func (l *FakeReader) WriteTo(w io.Writer) (n int64, err error) {
	size := int(l.size)
	bufferSize := len(sharedBytes)
	for size > 0 {
		tempBuffer := sharedBytes
		if size < bufferSize {
			tempBuffer = sharedBytes[0:size]
		}
		count, e := w.Write(tempBuffer)
		if e != nil {
			return int64(size), e
		}
		size -= count
	}
	return l.size, nil
}

func Readln(r *bufio.Reader) ([]byte, error) {
	var (
		isPrefix = true
		err      error
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return ln, err
}
