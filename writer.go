package lux

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

const (
	noWritten     = -1
	defaultStatus = http.StatusOK
)

type ResponseWriter interface {
	http.ResponseWriter
	http.Hijacker
	http.Flusher
	http.CloseNotifier

	Status() int
	Size() int
	WriteString(string) (int, error)
	Written() bool
	WriteHeaderNow()
	Pusher() http.Pusher
}

type responseWriter struct {
	http.ResponseWriter
	size   int
	status int

	conn         net.Conn
	header       http.Header
	headerSent   bool
	writer       *bufio.Writer
	hijackReader *bufio.Reader
}

var _ ResponseWriter = (*responseWriter)(nil)

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) reset(writer http.ResponseWriter) {
	w.ResponseWriter = writer
	w.size = noWritten
	w.status = defaultStatus
}

func (w *responseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *responseWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.Written() {
			// TODO print in debug mode
			return
		}
		w.status = code
	}
}

func (w *responseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
		if !w.headerSent {
			w.writeHeaders()
		}
	}
}

func (w *responseWriter) writeHeaders() {
	// Write status line
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", w.status, http.StatusText(w.status))
	w.writer.WriteString(statusLine)

	// Write headers
	for key, values := range w.header {
		for _, value := range values {
			headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
			w.writer.WriteString(headerLine)
		}
	}

	// Add Content-Length if not present but we know the size
	if w.header.Get("Content-Length") == "" && w.header.Get("Transfer-Encoding") == "" {
		w.writer.WriteString("\r\n")
	}

	w.writer.Flush()
	w.headerSent = true
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	w.WriteHeaderNow()
	n, err = w.writer.Write(data)
	w.writer.Flush()
	w.size += n
	return
}

func (w *responseWriter) WriteString(s string) (n int, err error) {
	w.WriteHeaderNow()
	n, err = w.writer.WriteString(s)
	w.writer.Flush()
	w.size += n
	return
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.size < 0 {
		w.size = 0
	}
	if w.headerSent {
		return nil, nil, fmt.Errorf("cannot hijack connection after headers have been written")
	}

	rw := bufio.NewReadWriter(w.hijackReader, w.writer)
	return w.conn, rw, nil
}

func (w *responseWriter) Flush() {
	w.WriteHeaderNow()
	w.writer.Flush()
}

func (w *responseWriter) CloseNotify() <-chan bool {
	// Implement a simple close notifier
	notify := make(chan bool, 1)

	// This is a simple implementation that may need to be enhanced
	// in a production environment
	go func() {
		// This will detect when the connection is closed
		var buf [1]byte
		_, err := w.conn.Read(buf[:])
		if err != nil {
			notify <- true
		}
	}()

	return notify
}

func (w *responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.size != noWritten
}

func (w *responseWriter) Pusher() http.Pusher {
	// Raw connections don't support HTTP/2 Push
	return nil
}

// NewResponseWriter creates a responseWriter from a net.Conn
func NewResponseWriter(conn net.Conn, req *http.Request) ResponseWriter {
	hijackReader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	w := &responseWriter{
		conn:         conn,
		header:       make(http.Header),
		status:       defaultStatus,
		size:         noWritten,
		writer:       writer,
		hijackReader: hijackReader,
	}

	// ResponseWriter is normally nil since we're creating this ourselves
	// and not wrapping an existing ResponseWriter

	return w
}

//const (
//	noWritten     = -1
//	defaultStatus = http.StatusOK
//)
//
//type ResponseWriter interface {
//	http.ResponseWriter
//	http.Hijacker
//	http.Flusher
//	http.CloseNotifier
//
//	Status() int
//	Size() int
//	WriteString(string) (int, error)
//	Written() bool
//	WriteHeaderNow()
//	Pusher() http.Pusher
//}
//
//type responseWriter struct {
//	http.ResponseWriter
//	size   int
//	status int
//
//	//TODO remove this later
//	conn       net.Conn
//	header     map[string]string
//	statusCode int
//	headerSent bool
//}
//
//var _ ResponseWriter = (*responseWriter)(nil)
//
//func (w *responseWriter) Unwrap() http.ResponseWriter {
//	return w.ResponseWriter
//}
//
//func (w *responseWriter) reset(writer http.ResponseWriter) {
//	w.ResponseWriter = writer
//	w.size = noWritten
//	w.status = defaultStatus
//}
//func (w *responseWriter) WriteHeader(code int) {
//	if code > 0 && w.statusCode != code {
//		if w.Written() {
//			//TODO print in debug mode
//			return
//		}
//		w.status = code
//	}
//
//}
//
//func (w *responseWriter) WriteHeaderNow() {
//	if !w.Written() {
//		w.size = 0
//		w.ResponseWriter.WriteHeader(w.status)
//	}
//}
//
//func (w *responseWriter) Write(data []byte) (n int, err error) {
//	w.WriteHeaderNow()
//	n, err = w.ResponseWriter.Write(data)
//	w.size += n
//	return
//}
//
//func (w *responseWriter) WriteString(s string) (n int, err error) {
//	w.WriteHeaderNow()
//	n, err = io.WriteString(w.ResponseWriter, s)
//	w.size += n
//	return
//}
//
//func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
//	if w.size < 0 {
//		w.size = 0
//	}
//	return w.ResponseWriter.(http.Hijacker).Hijack()
//}
//
//func (w *responseWriter) Flush() {
//	w.WriteHeaderNow()
//	w.ResponseWriter.(http.Flusher).Flush()
//}
//
//func (w *responseWriter) CloseNotify() <-chan bool {
//	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
//}
//
//func (w *responseWriter) Status() int {
//	return w.status
//}
//
//func (w *responseWriter) Size() int {
//	return w.size
//}
//
//func (w *responseWriter) Written() bool {
//	return w.size != noWritten
//}
//
//func (w *responseWriter) Pusher() http.Pusher {
//	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
//		return pusher
//	}
//	return nil
//}
//func newResponseWriter(conn net.Conn) *responseWriter {
//	return &responseWriter{conn: conn, header: make(map[string]string), statusCode: http.StatusOK}
//}
