package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

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

// Get the name of the track from the URL.
// Returns the name as a string.
func getTrackName(URL string) string {
	splittedURL := strings.Split(URL, "?dest-")
	baseURL := splittedURL[0]
	splittedBaseURL := strings.Split(baseURL, "/")
	return splittedBaseURL[len(splittedBaseURL)-1]
}

// Download the file from the URL.
// Error is nil if no error occured.
func downloadFile(URL string) (err error) {
	// Get the data.
	resp, err := http.Get(URL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Check the download dir and if it isn't exists create it.
	downloadDir := "Downloads"
	err = os.Mkdir(downloadDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return
	}

	// Create the file.
	filePath := "Downloads/" + getTrackName(URL)
	file, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)

	return
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

	err = downloadFile(links[0])
	if err != nil {
		log.Fatal("Cannot download the file: " + err.Error())
	}

	fmt.Println("DONE")
}
