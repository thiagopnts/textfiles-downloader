package main

import (
	"code.google.com/p/go-html-transform/h5"
	"code.google.com/p/go-html-transform/html/transform"
	"code.google.com/p/go.net/html"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Downloader struct {
	urlBase string
	dest    string
	workers int
}

type Job struct {
	url      string
	dest     string
	filename string
}

var jobsDone int = 0
var size int = 0

func New(workers int, url string, dest string) *Downloader {
	return &Downloader{workers: workers, urlBase: url, dest: dest}
}

func (d *Downloader) Start() {
	links := make(chan Job)
	var group sync.WaitGroup
	for i := 1; i <= d.workers; i++ {
		go func(name string, jobs <-chan Job) {
			for job := range jobs {
				fmt.Printf("[%s] Downloading %s...\n", name, job.url)
				err := download(job.url, filepath.Join(job.dest, job.filename))
				jobsDone++
//      bar := progress(jobsDone)
//			os.Stdout.Write([]byte(bar + "\r"))
//			os.Stdout.Sync()
				if err != nil {
					log.Fatal("Download failed", err)
				}
				group.Done()
			}
		}(fmt.Sprintf("WORKER %d", i), links)
	}
	list := fetchPage(d.urlBase, extractLinks)
	size = len(list)
	fmt.Printf("%d files found. Downloading...\n", size)
	for _, href := range list {
		links <- Job{url: d.urlBase + href, dest: d.dest, filename: href}
		group.Add(1)
	}
	group.Wait()
}

func fetchPage(url string, fn func(io.Reader) []string) []string {
	res, err := http.Get(url)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	return fn(res.Body)
}

func extractLinks(page io.Reader) []string {
	var links []string
	h, _ := h5.New(page)
	dom := transform.New(h)

	dom.Apply(transform.CopyAnd(func(node *html.Node) {
		if !strings.HasSuffix(node.Attr[0].Val, ".zip") && !strings.HasSuffix(node.Attr[0].Val, "tar.gz") && strings.ContainsAny(node.Attr[0].Val, ".") {
			links = append(links, node.Attr[0].Val)
		}
	}), "a")

	return links
}

func download(url string, filename string) error {
	outputFile, _ := os.Create(filename)
	defer outputFile.Close()

	res, err := http.Get(url)
	if err != nil {
		return errors.New("HTTP connection failed")
	}
	defer res.Body.Close()

	io.Copy(outputFile, res.Body)
	return nil
}

func bold(str string) string {
	return "\033[1m" + str + "\033[0m"
}

func progress(current int) string {
	prefix := strconv.Itoa(current) + " / " + strconv.Itoa(size)

	return bold(prefix)
}

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	workers := flag.Int("w", 1, "Number of workers")
	dest := flag.String("d", usr.HomeDir, "Directory to downloaded files")
	url := flag.String("u", "http://www.textfiles.com/programming/", "Url to download files")
	flag.Parse()

	d := New(*workers, *url, *dest)
	d.Start()
}
