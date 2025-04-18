package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

type HTTPClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

type CallOption struct {
	Method    string
	UploadUrl string
	Filename  string
	MimeType  string
	PairMap   map[string]string
	Header    map[string]string
}

func Call(req *fasthttp.Request) (resp *fasthttp.Response, err error) {
	resp = fasthttp.AcquireResponse()
	for k, v := range GetHeader() {
		req.Header.Set(k, v)
	}
	err = Hc.Do(req, resp)
	return
}

func upload_body(req *fasthttp.Request, fillBufferFunction func(w io.Writer) error, option *CallOption) (resp *fasthttp.Response, err error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	if err = fillBufferFunction(buf); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return
	}

	resp = fasthttp.AcquireResponse()
	req.SetBody(buf.Bytes())

	for k, v := range GetHeader() {
		req.Header.Set(k, v)
	}
	err = Hc.Do(req, resp)
	return
}

func upload_content(req *fasthttp.Request, fillBufferFunction func(w io.Writer) error, option *CallOption) (resp *fasthttp.Response, err error) {
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

	fileWriter, cpErr := body_writer.CreatePart(h)
	if cpErr != nil {
		log.Printf("error creating form file %s\n", cpErr.Error())
		return resp, cpErr
	}
	if err := fillBufferFunction(fileWriter); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return resp, err
	}
	multipartContentType := body_writer.FormDataContentType()
	if err = body_writer.Close(); err != nil {
		log.Printf("error closing body %s\n", err.Error())
		return resp, err
	}

	for k, v := range GetHeader() {
		req.Header.Set(k, v)
	}

	req.Header.Set("Content-Type", multipartContentType)
	req.SetBody(buf.Bytes())
	resp = fasthttp.AcquireResponse()
	err = Hc.Do(req, resp)
	if err != nil {
		err = fmt.Errorf("upload %s %d bytes to  %v", option.Filename, len(buf.Bytes()), err)
		return
	}
	return
}

func Upload(req *fasthttp.Request, reader io.Reader, option *CallOption) (resp *fasthttp.Response, err error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		err = fmt.Errorf("read input: %v", err)
		return
	}
	for i := 0; i < 3; i++ {
		if option.MimeType == "multipart/form-data" {
			resp, err = upload_content(req, func(w io.Writer) (err error) {
				_, err = w.Write(data)
				return
			}, option)
		} else {
			resp, err = upload_body(req, func(w io.Writer) (err error) {
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

func CloseReqResponse(req *fasthttp.Request, resp *fasthttp.Response) {
	if req != nil {
		fasthttp.ReleaseRequest(req)
	}
	if resp != nil {
		fasthttp.ReleaseResponse(resp)
	}
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
