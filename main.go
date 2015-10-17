package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
)

type visitTracker struct {
	mtx *sync.Mutex
	v   map[string]bool
}

var (
	webRoot   = flag.String("root", "", "Root web url to crawl")
	ft        = flag.String("filetype", ".zip", "Filetype to look for")
	filter    = flag.String("filter", "", "Regex filter to apply to URL")
	outputDir = flag.String("output", "", "Optional output directory")
	simulate  = flag.Bool("s", false, "Simulate, just print URL we would download")

	errInvalidResponse = errors.New("Invalid HTTP response")
	outDir             = "./"
	host               string
)

func init() {
	flag.Parse()
	if *webRoot == "" {
		log.Fatal("I require a webroot")
	}
	if *outputDir != "" {
		outDir = *outputDir
	}
}

func main() {
	var re *regexp.Regexp
	var err error
	visited := newVisitTracker()
	if *filter != "" {
		re, err = regexp.CompilePOSIX(*filter)
		if err != nil {
			log.Fatal("Failed to compile regex", err)
		}
	}

	url, err := url.ParseRequestURI(*webRoot)
	if err != nil {
		log.Fatal("Invalid root URL:", err)
	}
	host = url.Scheme + "://" + url.Host

	//probe root page
	rootPage, err := getPage(*webRoot)
	if err != nil {
		log.Fatal("Failed to get root page", err)
	}

	urls := extractURLs(rootPage)
	walkUrls(visited, urls, *webRoot, re, *ft)
}

func extractURLs(body string) []string {
	urls := []string{}
	re := regexp.MustCompile(`<A HREF="([^"]+)">`)
	match := re.FindAllStringSubmatch(body, -1)
	for i := range match {
		if len(match[i]) != 2 {
			continue
		}
		urls = append(urls, match[i][1])
	}
	return urls
}

func getUrlsFromPage(url string) ([]string, error) {
	body, err := getPage(url)
	if err != nil {
		return nil, err
	}
	return extractURLs(body), nil
}

func walkUrls(vt *visitTracker, urls []string, root string, filter *regexp.Regexp, ft string) {
	for i := range urls {
		var newUrl string
		u, err := url.ParseRequestURI(urls[i])
		if err != nil {
			fmt.Printf("Bad URL: %s\n", urls[i])
			continue
		}
		if u.String() == "/" {
			continue
		}
		if strings.HasSuffix(root, u.String()) {
			//parent URL, skipping
			fmt.Printf("Skipping parent: %s\n", u.String())
			continue
		}
		if strings.HasPrefix(u.String(), `/`) {
			//not a relative URL, correct
			newUrl = host + urls[i]
		} else {
			//relative URL
			newUrl = root + urls[i]
		}
		if !vt.Visited(newUrl) {
			vt.Visit(newUrl)
			//check if we recurse into it, or grab a file
			if strings.HasSuffix(urls[i], `/`) {
				//recurse in
				childUrls, err := getUrlsFromPage(newUrl)
				if err != nil {
					fmt.Printf("Failed to get %s: %v\n", newUrl, err)
					continue
				}
				walkUrls(vt, childUrls, newUrl, filter, ft)
			} else if strings.HasSuffix(urls[i], ft) {
				if filter != nil {
					if !filter.MatchString(newUrl) {
						continue
					}
				}
				fmt.Printf("Downloading %s ...", urls[i])
				if err := downloadFile(newUrl, urls[i], outDir); err != nil {
					fmt.Printf("Failed: %v\n", err)
				} else {
					fmt.Printf("DONE\n")
				}
			}
		}
	}
}

func downloadFile(url, filename, destination string) error {
	if *simulate {
		fmt.Printf("%s\n", url)
		return nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid status %s", resp.Status)
	}
	filename = path.Base(filename)
	fout, err := os.Create(path.Join(destination, filename))
	if err != nil {
		return err
	}
	defer fout.Close()
	io.Copy(fout, resp.Body)
	return nil
}

func getPage(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Invalid response %s", resp.Status)
	}
	bb := bytes.NewBuffer(nil)
	io.Copy(bb, resp.Body)
	return string(bb.Bytes()), nil
}

func newVisitTracker() *visitTracker {
	return &visitTracker{
		mtx: &sync.Mutex{},
		v:   make(map[string]bool, 256),
	}
}

func (vt *visitTracker) Visited(url string) bool {
	vt.mtx.Lock()
	defer vt.mtx.Unlock()
	_, ok := vt.v[url]
	return ok
}

func (vt *visitTracker) Visit(url string) error {
	vt.mtx.Lock()
	defer vt.mtx.Unlock()
	_, ok := vt.v[url]
	if ok {
		return errors.New("Already visited")
	}
	vt.v[url] = true
	return nil
}
