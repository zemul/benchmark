package main

import (
	"bufio"
	"bytes"
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

type UploadOption struct {
	Method    string
	UploadUrl string
	Filename  string
	MimeType  string
	PairMap   map[string]string
}

func Head(url string) (resp *http.Response, err error) {
	request, err := http.NewRequest("HEAD", url, nil)

	resp, err = Hc.Do(request)
	return
}

func Delete(url string) (resp *http.Response, err error) {
	request, err := http.NewRequest("DELETE", url, nil)
	resp, err = Hc.Do(request)
	return
}

func Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	resp, err = Hc.Do(req)
	return
}

func upload_body(fillBufferFunction func(w io.Writer) error, option *UploadOption) (resp *http.Response, err error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	if err = fillBufferFunction(buf); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return
	}

	req, postErr := http.NewRequest(option.Method, option.UploadUrl, bytes.NewReader(buf.Bytes()))
	if postErr != nil {
		err = fmt.Errorf("create upload request %s: %v", option.UploadUrl, postErr)
		return
	}
	req.Header.Set("Content-Type", option.MimeType)
	for k, v := range option.PairMap {
		req.Header.Set(k, v)
	}

	resp, err = Hc.Do(req)
	return
}

func upload_content(fillBufferFunction func(w io.Writer) error, option *UploadOption) (resp *http.Response, err error) {
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
		return resp, cp_err
	}
	if err := fillBufferFunction(file_writer); err != nil {
		log.Printf("error copying data %s\n", err.Error())
		return resp, err
	}
	content_type := body_writer.FormDataContentType()
	if err = body_writer.Close(); err != nil {
		log.Printf("error closing body %s\n", err.Error())
		return resp, err
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

	resp, err = Hc.Do(req)
	if err != nil {
		err = fmt.Errorf("upload %s %d bytes to  %v", option.Filename, option.UploadUrl, err)
		return
	}
	return
}

func Upload(reader io.Reader, option *UploadOption) (resp *http.Response, err error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		err = fmt.Errorf("read input: %v", err)
		return
	}
	for i := 0; i < 3; i++ {
		if option.MimeType == "multipart/form-data" {
			resp, err = upload_content(func(w io.Writer) (err error) {
				_, err = w.Write(data)
				return
			}, option)
		} else {
			resp, err = upload_body(func(w io.Writer) (err error) {
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
	if resp != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
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
