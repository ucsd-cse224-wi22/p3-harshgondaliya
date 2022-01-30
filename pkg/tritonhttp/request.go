package tritonhttp

import (
	"bufio"
	"fmt"
	"log"
	"regexp"
	"strings"
)

type Request struct {
	Method string // e.g. "GET"
	URL    string // e.g. "/path/to/a/file"
	Proto  string // e.g. "HTTP/1.1"

	// Header stores misc headers excluding "Host" and "Connection",
	// which are stored in special fields below.
	// Header keys are case-incensitive, and should be stored
	// in the canonical format in this map.
	Header map[string]string

	Host  string // determine from the "Host" header
	Close bool   // determine from the "Connection" header
}

// ReadRequest tries to read the next valid request from br.
//
// If it succeeds, it returns the valid request read. In this case,
// bytesReceived should be true, and err should be nil.
//
// If an error occurs during the reading, it returns the error,
// and a nil request. In this case, bytesReceived indicates whether or not
// some bytes are received before the error occurs. This is useful to determine
// the timeout with partial request received condition.

// func ReadRequest(br *bufio.Reader) (req *Request, bytesReceived bool, err error) {
// 	panic("todo")

// 	// Read start line

// 	// Read headers

// 	// Check required headers

// 	// Handle special headers
// }
func ReadRequest(br *bufio.Reader) (req *Request, bytesReceived bool, err error){
	req = &Request{}
	req.Header = make(map[string]string)
	//read start line
	line, err := ReadLine(br) // always returns all data read till \r\n or till the pt of err 
	if err != nil{             // if EOF came, still the past char read are there in line
		if len(line) > 0{ 
			bytesReceived = true
		}else{
			bytesReceived = false // case where directly EOF is reached. Nothing there to be read on connection
		}
		return nil, bytesReceived, err 
	}
	bytesReceived = true // no error so at least 1 byte has been read
	startLineFields, err := parseRequestStartLine(line)
	if err!= nil{ 
		return nil, bytesReceived, badStringError("malformed start line", line)
	}
	req.Method = startLineFields[0]
	if !validMethod(req.Method){ 
		return nil, bytesReceived, badStringError("invalid method", req.Method)
	}
	req.URL = startLineFields[1]
	if !validURL(req.URL){ 
		return nil, bytesReceived, badStringError("invalid URL", req.URL)
	}
	if req.URL[len(req.URL)-1] == '/'{
		req.URL += "index.html"
	}
	req.Proto = startLineFields[2]
	if !validProto(req.Proto){ 
		return nil, bytesReceived, badStringError("invalid Proto", req.Proto)
	}
	for{
		line, err := ReadLine(br)		
		if err != nil{
			return nil, bytesReceived, err // bytesReceived true because start line is already received
		}
		if line == ""{
			break // header end
		}
		header, err := parseRequestHeader(line)
		if !validHeader(header){ 
			return nil, bytesReceived, badStringError("invalid Header", header)
		}
		header = CanonicalHeaderKey(header)
		value := parseRequestValue(line)
		if header == "Host"{
			req.Host = value
		} 
		if header == "Connection"{
			req.Close = (value == "close")
		}
		if header != "Connection" && header != "Host"{
			req.Header[header] = value
		}
		log.Println("Read line from request", line)
	}
	if req.Host == ""{
		return nil, bytesReceived, badStringError("Host Header Missing", "")
	}
	return req, bytesReceived, nil // TODO: think more whether to return true
}
func badStringError(what, val string) error {
	return fmt.Errorf("%s %q", what, val)
}
func parseRequestStartLine(line string) ([]string, error){
	startLineFields := strings.SplitN(line, " ", 3) // splitN takes each separator as reference and returns stuff that is there between a given separator and previous one
	// Thus, GET<spc><spc><spc><spc><spc>/path will have six elements in startLineFields if no restrictions are placed
	// limit means consider that many separators and then stop splitting based on separators	
	if len(startLineFields) != 3{ // less than 2 meaning definition not satisfied.
		return nil, fmt.Errorf("could not parse the request line, got startLineFields %v", startLineFields) 
	}
	return startLineFields, nil
}
func validMethod(method string) bool{
	return method == "GET"
}
func validURL(url string) bool{
	if len(url) > 0{
		return url[0] == '/'
	} else {
		return false
	}
}
func validProto(proto string) bool{
	if len(proto) > 0{
		return proto == "HTTP/1.1"
	} else {
		return false
	}
}
func parseRequestHeader(line string) (string, error){
	headerLineFields := strings.SplitN(line, ":", 2)	
	if len(headerLineFields) != 2{ // less than 2 meaning malformed
		return "", fmt.Errorf("could not parse the request header , got header %v", headerLineFields) 
	}
	return headerLineFields[0], nil
}
func validHeader(header string) bool{
	if len(header) > 0{
		re := regexp.MustCompile("^[a-zA-Z0-9-]*$")
  		return re.MatchString(header)
	} else {
		return false
	}
}
func parseRequestValue(line string) (string){
	headerLineFields := strings.SplitN(line, ":", 2) // we only do this if we already know that we have key value pair separated by colon
	afterColonString := headerLineFields[1]
	var nonSpaceCharIndex int
	for i, ch := range afterColonString {		
		if ch != ' '{
			nonSpaceCharIndex = i
			break
		}
	}
	value := afterColonString[nonSpaceCharIndex:]
	return value
}
