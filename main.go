package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"golang.org/x/net/html"
)

// Store the number of written bytes to the disk.
// Implements the io.Writer interface.
type fileWriteCounter struct {
	TotalSize uint64
	Written   uint64
	Channel   chan<- uint64
}

func (fwc *fileWriteCounter) Write(data []byte) (int, error) {
	// Get the length of the current chunk.
	size := len(data)
	// Increment the written bytes.
	fwc.Written += uint64(size)
	// Send it to the channel.
	fwc.Channel <- fwc.Written

	return size, nil
}

// Download the HTML.
// If no error happened returns the body of the response.
func getHTML(URL string) (string, error) {
	query := "/episodes?active=true&limit=10000&order=newest&offset=1"

	// Make the GET request.
	resp, err := http.Get(URL + query)
	if err != nil || resp.StatusCode != 200 {
		return "", errors.New("can't download the HTML")
	}
	// Close the body at the end of the function.
	defer resp.Body.Close()

	// Read the body of the response.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("invalid response body")
	}

	return string(body), nil
}

// Parse the HTML.
// If no error happened returns a string list with the MP3 URLs.
func parseHTML(body string) ([]string, error) {
	// Links to the MP3-s.
	links := make([]string, 0)

	// Tokenize the HTML.
	tokenizer := html.NewTokenizer(strings.NewReader(body))

	// Iterate over the tokens.
	for {
		// Get the next token.
		tt := tokenizer.Next()
		// Choose action based on the token.
		switch tt {
		// Error occurd or EOF.
		case html.ErrorToken:
			// If no link found return an error.
			if len(links) == 0 {
				return links, errors.New("no mp3 link found")
			}
			return links, nil
		// Opening element.
		case html.StartTagToken:
			// Get the current token.
			token := tokenizer.Token()

			// We are looking for the anchor elements.
			if token.Data == "a" {
				var class, href string

				// Iterate over the attributes of the element.
				for _, attribute := range token.Attr {
					if attribute.Key == "class" {
						class = attribute.Val
					} else if attribute.Key == "href" {
						href = attribute.Val
					}
				}

				// If the class is appropriate save the link in the href attr.
				if strings.Contains(class, "action normal playable") {
					links = append(links, href)
				}
			}
		}
	}
}

// Get the name of the file from the URL.
// Returns the name as a string.
func getFileName(URL string) string {
	splittedURL := strings.Split(URL, "?dest-")
	baseURL := splittedURL[0]
	splittedBaseURL := strings.Split(baseURL, "/")
	return splittedBaseURL[len(splittedBaseURL)-1]
}

// Download the file from the URL.
func downloadFile(URL string, fileName string, fileSizeChan chan<- uint64, writtenBytesChan chan<- uint64) {
	// Get the data.
	resp, err := http.Get(URL)
	// Check error
	if err != nil {
		return
	}
	// and status code.
	if resp.StatusCode != 200 {
		err = errors.New("bad status code: " + string(resp.StatusCode))
		return
	}
	defer resp.Body.Close()

	// Get the size of the file
	fileSize, err := strconv.Atoi(resp.Header["Content-Length"][0])
	if err != nil {
		fileSizeChan <- 0
	}
	// then send it to the channel.
	fileSizeChan <- uint64(fileSize)

	// Check the download dir and if it isn't exists create it.
	downloadDir := "Downloads"
	err = os.Mkdir(downloadDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return
	}

	// Create the file.
	filePath := "Downloads/" + fileName
	file, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Create the counter and
	counter := fileWriteCounter{Channel: writtenBytesChan}
	// start to copy the file.
	_, err = io.Copy(file, io.TeeReader(resp.Body, &counter))

	close(writtenBytesChan)
}

// Print the output to the standard output.
func printStatus(fileSize uint64, writtenBytes uint64) {
	// Function the convert the bytes to megabyte.
	toKiloBytess := func(bytes uint64) uint64 {
		return bytes / 1024
	}

	// Get the size of the terminal.
	width, _, _ := terminal.GetSize(0)

	// Get the status percentage.
	percentage := int(writtenBytes * 100 / fileSize)
	// Get the number of completed marks in the terminal UI.
	markCount := percentage / 5

	// Go back to the start of the current line.
	fmt.Printf("\r")

	// Create the line.
	line := fmt.Sprintf("[%s%s] %d/%d KB", strings.Repeat("=", markCount), strings.Repeat(".", 20-markCount), toKiloBytess(writtenBytes), toKiloBytess(fileSize))

	// Print it and fill the remaining space with spaces.
	fmt.Printf("%s%s", line, strings.Repeat(" ", width-len(line)))
}

func main() {
	var URL string
	fmt.Print("Enter base player.fm URL: ")
	fmt.Scanln(&URL)

	body, err := getHTML(URL)
	if err != nil {
		log.Fatal("Error while loading the page: " + err.Error())
	}

	links, err := parseHTML(body)
	if err != nil {
		log.Fatal("Error while parsing the HTML: " + err.Error())
	}

	fileSizeChan := make(chan uint64)
	// Download the files from the links one-by-one sequentially.
	for i, link := range links {
		writtenBytes := make(chan uint64)
		// Get the name of the file from the URL.
		fileName := getFileName(link)
		// Start the file download.
		go downloadFile(link, fileName, fileSizeChan, writtenBytes)

		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(links), fileName, link)

		// Get the size of the file.
		fileSize := <-fileSizeChan

		// While the channel is open
		for {
			writtenBytes, more := <-writtenBytes
			if !more {
				break
			}

			printStatus(fileSize, writtenBytes)
		}
		// New line after complete.
		fmt.Println()
	}

	fmt.Println("DONE")
}
