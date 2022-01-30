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
		req, _, err := ReadRequest(br) // TODO: handle partial bytesReceived case
		
		// Handle EOF
		if errors.Is(err, io.EOF){
			log.Printf("Connection closed by %v", conn.RemoteAddr())
			_ = conn.Close()
			return
		}
		
		// Handle timeout
		if err, ok := err.(net.Error); ok && err.Timeout(){
			log.Printf("Connection to %v timed out", conn.RemoteAddr())
			_ = conn.Close()
			return
		}

		// Handle bad request
		if err != nil{
			log.Printf("Handle bad reqest for error: %v", err)
			res := &Response{}
			res.HandleBadRequest()
			_ = res.WriteTwo(conn)
			_ = conn.Close()
			return // after a single bad request no need to handle subsequent requests
		}
		// Handle good request
		log.Printf("Handle good request %v", req)
		res := s.HandleGoodRequest(req)
		err = res.WriteTwo(conn)
		if err != nil{
			log.Println(err)
		}
		// Close conn if requested
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

// HandleNotFound prepares res to be a 404 Not Found response
// ready to be written back to client.
func (res *Response) HandleNotFound(req *Request) {
	log.Printf("hi")
}

// HandleGoodRequest handles the valid req and generates the corresponding res.
func (s *Server) HandleGoodRequest(req *Request) (res *Response) {
	res = &Response{}
	res.HandleOK(req, "hello-world.txt") // TODO: to be altered
	res.FilePath = filepath.Join(s.DocRoot, "hello-world.txt")
	return res
}

// HandleOK prepares res to be a 200 OK response
// ready to be written back to client.
func (res *Response) HandleOK(req *Request, path string) {
	res.init()
	res.StatusCode = statusOK
}
// HandleBadRequest prepares res to be a 400 Bad Request response
// ready to be written back to client.
func (res *Response) HandleBadRequest() {
	res.init()
	res.StatusCode = statusMethodNotAllowed
	res.FilePath = ""
}
