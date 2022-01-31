package tritonhttp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// DocRoot specifies the path to the directory to serve static files from.
	DocRoot string
}

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {
	// validate the configuration of server
	if err := s.ValidateServerSetup(); err!= nil{
		return fmt.Errorf("server is not setup correctly %v", err)
	}
	log.Println("Server setup valid!")

	// listen for connections
	ln, err := net.Listen("tcp", s.Addr)
	if err!=nil{
		return err
	}
	log.Println("Listening on", ln.Addr())

	// ensure that listener is closed when we are done
	defer func(){
		err = ln.Close()
		if err != nil{
			log.Println("Error in closing connection", err)
		}
	}()

	for{
		conn, err := ln.Accept()
		if err!= nil{
			continue
		}
		log.Println("Accepted connection from ", conn.RemoteAddr())
		go s.HandleConnection(conn)
	}
}

// HandleConnection reads requests from the accepted conn and handles them.
func (s *Server) HandleConnection(conn net.Conn) {
	br := bufio.NewReader(conn)
	// Hint: use the other methods below

	for {
		// Set timeout
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			log.Printf("Failed to set timeout for the connection %v", conn)
			_ = conn.Close()
			return
		}
		// Try to read next request
		req, bytesReceived, readErr := ReadRequest(br) // TODO: handle partial bytesReceived case
		
		// Handle EOF
		if errors.Is(readErr, io.EOF){
			log.Printf("Connection closed by %v", conn.RemoteAddr())
			_ = conn.Close()
			return
		}
		
		// Handle timeout
		if err, ok := readErr.(net.Error); ok && err.Timeout(){
			if bytesReceived { // connection timed out but partial bytes received
 				log.Printf("Timeout occured and Partial Bytes Read: %v", readErr)
				res := &Response{}
				res.Header = make(map[string]string)
				res.HandleBadRequest()
				log.Println(res)
				werr := res.Write(conn)
				if werr != nil{
					log.Println("Error while writing reponse for bad request", werr)
				}
				_ = conn.Close()
				return // after a single bad request no need to handle subsequent requests
			}else {
				log.Printf("Connection to %v timed out. No Bytes were read", conn.RemoteAddr())
				_ = conn.Close()
				return
			} 
		}

		// Handle bad request
		if readErr != nil{
			log.Printf("Handle bad reqest for error: %v", readErr)
			res := &Response{}
			res.Header = make(map[string]string)
			res.HandleBadRequest()
			log.Println(res)
			werr := res.Write(conn)
			if werr != nil{
				log.Println("Error while writing reponse for bad request", werr)
			}
			_ = conn.Close()
			return // after a single bad request no need to handle subsequent requests
		}
		// Handle good request
		log.Printf("Handle good request %v", req)
		res := s.HandleGoodRequest(req)
		log.Println(res)
		werr := res.Write(conn)
		if werr != nil{
			log.Println(werr)
		}
		// Close conn if requested
		v, exists := res.Header["Connection"]
		if exists && v == "close"{
			log.Println("Closing connection because close received")
			_ = conn.Close()
			return
		}
	}
}

func (s *Server) ValidateServerSetup() error {
	fi, err := os.Stat(s.DocRoot)
	
	if os.IsNotExist(err){
		return err
	}
	if !fi.IsDir(){
		return fmt.Errorf("doc root %q is not a directory", s.DocRoot)
	}
	return nil
}

// HandleGoodRequest handles the valid req and generates the corresponding res.
func (s *Server) HandleGoodRequest(req *Request) (res *Response) {
	res = &Response{}
	res.Header = make(map[string]string)
	if req.URL[len(req.URL)-1] == '/'{ // we are sure that req is at least one char long so can access
		req.URL += "index.html" // moved here because unit test expects it to be here
	}
	cleanedPath := filepath.Join(s.DocRoot, req.URL)
	fi, err := os.Stat(cleanedPath)
	log.Println("Cleaned Path", cleanedPath)
	var maliciousURL bool
	if len(cleanedPath) >= len(s.DocRoot){
		if strings.HasPrefix(cleanedPath[:len(s.DocRoot)], s.DocRoot){
			maliciousURL = false
		} else{
			maliciousURL = true
		}
	} else {
		// if (len(cleanedPath) + 1) == len(s.DocRoot) {  // if s.DocRoot is "testdata/" and URL is "/" then result of clean is "testdata"
		// 	maliciousURL = (s.DocRoot[len(s.DocRoot)-1] != '/') // wrong assumption. if we have / then our code has already inserted /index.html
		// 	log.Println("NOT Malicious")
		// } else {
		// 	maliciousURL = true
		// }
		maliciousURL = true
	}
	log.Println("malicious URL: ", maliciousURL)
	if os.IsNotExist(err) || maliciousURL{
		res.HandleNotFound(req)
	} else {
		res.HandleOK(req, cleanedPath)
		res.Header["Last-Modified"] = FormatTime(fi.ModTime())
		res.Header["Content-Length"] = fmt.Sprint(fi.Size())
	}
	return res
}

// HandleOK prepares res to be a 200 OK response
// ready to be written back to client.
func (res *Response) HandleOK(req *Request, cleanedPath string) {
	res.init()
	res.StatusCode = statusOK
	if req.Close {
		res.Header["Connection"] = "close"
	}
	res.Header["Date"] = FormatTime(time.Now())
	lastSlashIndex := strings.LastIndex(cleanedPath, "/") // we will always have a '/' embedded when we reach this part of code
	stringAfterLastSlash := cleanedPath[lastSlashIndex:]
	dotIndex := strings.LastIndex(stringAfterLastSlash, ".")
	if dotIndex == -1 {
		res.Header["Content-Type"] = MIMETypeByExtension("") 
	} else{
		res.Header["Content-Type"] = MIMETypeByExtension(stringAfterLastSlash[dotIndex:])
	}
	res.Request = req
	res.FilePath = cleanedPath
}
// HandleBadRequest prepares res to be a 400 Bad Request response
// ready to be written back to client.
func (res *Response) HandleBadRequest() {
	res.init()
	res.StatusCode = statusBadRequest
	res.Header["Date"] = FormatTime(time.Now())
	res.Header["Connection"] = "close"
	res.Request = nil
	res.FilePath = ""
}

// HandleNotFound prepares res to be a 404 Not Found response
// ready to be written back to client.
func (res *Response) HandleNotFound(req *Request) {
	res.init()
	res.StatusCode = statusNotFound
	res.Header["Date"] = FormatTime(time.Now())
	if req.Close {
		res.Header["Connection"] = "close"
	}
	res.Request = req
	res.FilePath = ""
}
