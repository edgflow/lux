package lux

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"golang.org/x/net/http/httpguts"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
)

type Request struct {
	Method string

	URL *url.URL

	Proto      string //"http/1.0
	ProtoMajor int
	ProtoMinor int

	Header map[string]string

	Body io.ReadCloser

	GetBody func() (io.ReadCloser, error)

	ContentLength int64

	Close bool

	Host string

	Form url.Values

	PostForm url.Values

	MultipartForm *multipart.Form

	RemoteAddr string

	RequestURI string

	Cancel <-chan struct{}
	// Response is the redirect response which caused this Request
	// to be created. This field is only populated during client
	// redirects.
	Response *Response

	ctx        context.Context
	FileHeader *multipart.FileHeader
	Boundary   string
}

func ReadRequest(b *bufio.Reader) (*Request, error) {
	req, err := readRequest(b)
	if err != nil {
		return nil, err
	}
	delete(req.Header, "Host")
	return req, nil
}

func readRequest(b *bufio.Reader) (req *Request, err error) {
	//tp := b
	req = new(Request)

	//First line : Get /index/html HTTP/1.0
	var s string
	if line, _, err := b.ReadLine(); err == nil {
		s = string(line)
	}
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	var ok bool
	req.Method, req.RequestURI, req.Proto, ok = parseRequestLine(s)
	if !ok {
		return nil, badStringError("maldormed HTTP Request", s)
	}

	rawurl := req.RequestURI
	if req.ProtoMajor, req.ProtoMinor, ok = ParseHttpVersion(req.Proto); !ok {
		return nil, badStringError("malformed http version", req.Proto)
	}

	justAuthority := req.Method == "CONNECT" && !strings.HasPrefix(rawurl, "/")
	if justAuthority {
		rawurl = "http://" + rawurl
	}

	if req.URL, err = url.ParseRequestURI(rawurl); err != nil {
		return nil, err
	}

	header := make(map[string]string)
	for {
		data, _, err := b.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line := string(data)
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		headerParts := strings.SplitN(line, ":", 2)
		if len(headerParts) == 2 {
			header[headerParts[0]] = strings.TrimSpace(headerParts[1])
		}
	}
	req.Header = header
	//TODO
	//close

	if len(req.Header["Host"]) > 1 {
		//return nil, fmt.Errorf("too many Host Headers")
	}

	req.Host = req.URL.Host
	if req.Host == "" {
		req.Host = req.Header["Host"]
	}

	req.Close = shouldClose(req.ProtoMajor, req.ProtoMinor, req.Header, false)
	// Read Content-Length
	var contentLength int64
	if val, ok := header["Content-Length"]; ok {
		contentLength, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	req.ContentLength = contentLength

	// Read the body - at this point the buffer contains only the body
	var bodyData []byte
	if contentLength > 0 {
		bodyData = make([]byte, contentLength)
		n, err := io.ReadFull(b, bodyData)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				// Use what we managed to read
				bodyData = bodyData[:n]
				fmt.Printf("Partial read: got %d of %d bytes\n", n, contentLength)
			} else {
				return nil, fmt.Errorf("error reading body: %w", err)
			}
		}
	} // Set body and GetBody function
	req.Body = io.NopCloser(bytes.NewReader(bodyData))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyData)), nil
	}
	contentType, ok := header["Content-Type"]
	// Process multipart form if needed
	if ok && strings.HasPrefix(contentType, "multipart/form-data") {
		parts := strings.Split(contentType, "boundary=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid multipart boundary")
		}

		req.Boundary = parts[1]

		// Create a new reader from the body data for multipart processing
		multipartReader := bytes.NewReader(bodyData)
		req.FileHeader, err = parseMultipartForm(multipartReader, parts[1])
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(contentType, "application/octet-stream") {
		// For application/octet-stream, the entire body is the content
		// You already have it in bodyData
		// If you need to store it somewhere specific:
		req.FileHeader = &multipart.FileHeader{
			Filename: "file", // Default name if none provided
			Size:     int64(len(bodyData)),
			Header:   make(map[string][]string),
		}

		// If you have a separate field for file content
		// req.FileContent = bodyData
	}

	return req, nil
}

func ParseHttpVersion(vers string) (major, minor int, ok bool) {
	switch vers {
	case "HTTP/1.1":
		return 1, 1, true
	case "HTTP/1.0":
		return 1, 1, true
	default:
		return 0, 0, false
	}
}

func parseRequestLine(line string) (method, requestURI, proto string, ok bool) {
	method, rest, ok1 := strings.Cut(line, " ")
	requestURI, proto, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 {
		return "", "", "", false
	}
	return method, requestURI, proto, true
}

func badStringError(what, val string) error {
	return fmt.Errorf("%s %q", what, val)
}
func shouldClose(major, minor int, header map[string]string, removeCloseHeader bool) bool {
	if major < 1 {
		return true
	}
	conv := header["Connection"]
	hasClose := httpguts.HeaderValuesContainsToken([]string{conv}, "close")
	if major == 1 && minor == 0 {
		if major == 1 && minor == 0 {
			return hasClose || !httpguts.HeaderValuesContainsToken([]string{conv}, "keep-alive")
		}
	}
	if hasClose && removeCloseHeader {
		delete(header, "Connection")
	}
	return hasClose
}

func parseMultipartForm(body io.Reader, boundary string) (*multipart.FileHeader, error) {
	reader := multipart.NewReader(body, boundary)

	// Loop through the parts
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break // No more parts
		}
		if err != nil {
			return nil, err
		}

		// Check if this part contains a file
		if part.FileName() != "" {
			// Read file content into a buffer (for size determination)
			buf := &bytes.Buffer{}
			_, err := io.Copy(buf, part)
			if err != nil {
				return nil, err
			}

			// Create a FileHeader-like structure
			fileHeader := &multipart.FileHeader{
				Filename: part.FileName(),
				Size:     int64(buf.Len()),
				Header:   part.Header,
			}
			return fileHeader, nil
		}
	}

	return nil, fmt.Errorf("no file found in multipart form")
}

func extractFileContent(bodyData []byte, boundary string) (string, error) {
	// Create a reader for the multipart data
	reader := multipart.NewReader(bytes.NewReader(bodyData), boundary)

	// Process each part
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break // No more parts
		}
		if err != nil {
			return "", err
		}

		// If this part has a filename, it's a file upload
		if part.FileName() != "" {
			// Read the file content
			content, err := io.ReadAll(part)
			if err != nil {
				return "", err
			}

			// Return the content as a string
			return string(content), nil
		}
	}

	return "", fmt.Errorf("no file content found")
}
