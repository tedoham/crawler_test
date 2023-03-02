package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

type Crawler struct {
	visited map[string]bool
	mu      sync.Mutex
}

func NewCrawler() *Crawler {
	return &Crawler{
		visited: make(map[string]bool),
	}
}

func (c *Crawler) IsVisited(link string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	visited := c.visited[link]
	c.visited[link] = true
	return visited
}

func Download(link string, destDir string) error {
	resp, err := http.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status: %s", resp.Status)
	}

	u, err := url.Parse(link)
	if err != nil {
		return err
	}

	dir := path.Join(destDir, u.Hostname())
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	var filename string
	if u.Path == "" || strings.HasSuffix(u.Path, "/") {
		filename = "index.html"
	} else {
		filename = path.Base(u.Path)
	}

	filepath := path.Join(dir, filename)
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded: %s\n", link)
	return nil
}

func ExtractLinks(pageUrl string, page io.Reader) ([]string, error) {
	links := []string{}
	z := html.NewTokenizer(page)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.StartTagToken || tt == html.SelfClosingTagToken {
			token := z.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						link := attr.Val
						u, err := url.Parse(link)
						if err == nil && u.Hostname() == "" && !strings.HasPrefix(u.Path, "#") {
							link = fmt.Sprintf("%s://%s%s", u.Scheme, pageUrl, u.Path)
						}
						if u, err := url.Parse(link); err == nil && u.Hostname() == pageUrl {
							links = append(links, link)
						}
					}
				}
			}
		}
	}
	return links, nil
}

func Crawl(url string, destDir string, c *Crawler) error {
	if c.IsVisited(url) {
		return nil
	}

	err := Download(url, destDir)
	if err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	links, err := ExtractLinks(url, resp.Body)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			Crawl(link, destDir, c)
		}(link)
	}
	wg.Wait()

	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go [starting URL] [destination directory]")
		os.Exit(1)
	}

	startingURL := os.Args[1]
	destDir := os.Args[2]

	c := NewCrawler()

	err := Crawl(startingURL, destDir, c)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
