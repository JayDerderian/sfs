package transfer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	svc "github.com/sfs/pkg/service"
)

// struct to use for populating context.Context
type FileContext struct {
	Name     string
	OwnerID  string
	Path     string
	Checksum string
}

func NewFileContext(name string, ownerID string, path string, cs string) FileContext {
	return FileContext{
		Name:     name,
		OwnerID:  ownerID,
		Path:     path,
		Checksum: cs,
	}
}

func (fctx FileContext) IsEmpty() bool {
	return fctx.Name == "" && fctx.OwnerID == "" && fctx.Path == "" && fctx.Checksum == ""
}

// transfer handles the uploading and downloading of individual files.
// transfer operations are intended to run in their own goroutine as part
// of sync operations with the server
type Transfer struct {
	Start  time.Time
	Buffer *bytes.Buffer

	// dedicated listener for downloads
	Listener func(network string, address string) (net.Listener, error)
	Port     int // port to listen to for downloads

	Client *http.Client
}

func NewTransfer(port int) *Transfer {
	return &Transfer{
		Start:    time.Now().UTC(),
		Buffer:   new(bytes.Buffer),
		Listener: net.Listen,
		Port:     port,
		Client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (t *Transfer) PrepareReq(method, contentType, destURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, destURL, t.Buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	return req, nil
}

// prepare and transfer a file for upload or download
//
// intended to run within its own goroutine
func (t *Transfer) Upload(method string, file *svc.File, destURL string) error {
	bodyWriter := multipart.NewWriter(t.Buffer)
	defer bodyWriter.Close()

	// create a form file field for the file
	fileWriter, err := bodyWriter.CreateFormFile("myFile", filepath.Base(file.Path))
	if err != nil {
		return err
	}

	// load data into file writer, then prepare and send the request to the destination
	if len(file.Content) == 0 {
		file.Load()
	}
	if _, err = fileWriter.Write(file.Content); err != nil {
		return fmt.Errorf("failed to retrieve file data: %v", err)
	}
	req, err := t.PrepareReq(method, bodyWriter.FormDataContentType(), destURL)
	if err != nil {
		return err
	}

	// add file info context to request
	fileCtx := NewFileContext(file.Name, file.OwnerID, file.Path, file.CheckSum)
	ctx := context.WithValue(req.Context(), File, fileCtx)

	// upload and confirm success
	log.Printf("[INFO] uploading %v ...", filepath.Base(file.Path))
	resp, err := t.Client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-OK status: %v", resp.Status)
	}
	log.Printf("[INFO] ...done")

	return nil
}

// download a file from the given URL.
//
// intended to run in its own goroutine
func (t *Transfer) Download(destPath string, fileURL string) error {
	// listen for server requests
	ln, err := t.Listener("tcp", fmt.Sprint(t.Port))
	if err != nil {
		return fmt.Errorf("failed to start client listener: %v", err)
	}
	defer ln.Close()

	// blocks until connection is established
	log.Print("[INFO] listening for download request...")
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("failed to create connection: %v", err)
	}
	defer conn.Close()

	// get file name & create local file
	fileNameBuffer := make([]byte, 0, 1024)
	n, err := conn.Read(fileNameBuffer)
	if err != nil {
		return fmt.Errorf("failed to read file name from server: %v", err)
	}
	fileName := string(fileNameBuffer[:n])
	file, err := os.Create(filepath.Join(destPath, fileName))
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// start downloading
	log.Printf("[INFO] downloading file %v ...", file)
	buffer := make([]byte, 0, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("failed to receive file data from server: %v", err)
			}
			break
		}
		_, err = file.Write(buffer[:n])
		if err != nil {
			return fmt.Errorf("[ERROR] failed to write file data: %v", err)
		}
	}
	log.Printf("...done")

	return nil
}
