package s3

import (
	"encoding/xml"
	"net/http"
)

type xmlOwner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func writeS3Error(w http.ResponseWriter, code, message string, status int) {
	writeXML(w, status, s3Error{Code: code, Message: message})
}

func writeXML(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(v)
}
