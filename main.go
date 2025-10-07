package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"bytes"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

type SpiderOptions struct{
	OutputFolder string
	BaseUrl      string
	Verbose      bool
	RecurseLevel int
	WorkerCount  int
	Timeout      time.Duration
}

func RecurseStatus(argList []string) (bool, error) {
	recurse := false
	found := false
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-r" {
			if found {
				return false, errors.New("error: -r flag specified multiple time")
			}
			found = true
			recurse = true
		}
	}
	return recurse, nil
}

func getRecurseLevel(argList []string) (int, error) {
	recurseLevel := 0
	found := false
	isRecurse, err := RecurseStatus(argList)
	if err != nil {
		return 0, err
	}
	if isRecurse {
		recurseLevel = 5
	}
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-l" {
			if found {
				return 0, errors.New("error: -l flag specified multiple time")
			} else if !isRecurse {
				return 0, errors.New("error: -l flag without associated -r flag")
			}
			found = true

			if i+1 < len(argList) {
				level, err := strconv.Atoi(argList[i+1])
				if err != nil || level < 1 || level > 20 {
					return 0, fmt.Errorf("error: invlid value for -l flag '%s'", argList[i+1])
				}
				recurseLevel = level
			} else {
				return 0, errors.New("error: missing value for -l flag")
			}
		}
	}
	return recurseLevel, nil
}

func getOutputFolder(argList []string) (string, error) {
	outputFolder := "./data/"
	found := false
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-p" {
			if found {
				return "", errors.New("error: -p flag specified multiple time")
			}
			found = true

			if i+1 < len(argList) {
				outputFolder = argList[i+1]
			} else {
				return "", errors.New("error: missing value for -p flag")
			}
		}
	}
	if info, err := os.Stat(outputFolder); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(outputFolder, 0755); err != nil {
				return "", fmt.Errorf("error: could not create directory '%s'", outputFolder)
			}
		} else {
			return "", fmt.Errorf("error: could not access '%s', %v", outputFolder, err)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("error: '%s' exists but is not a directory", outputFolder)
	} else {
		mode := info.Mode()
		if mode&0200 == 0 {
			return "", fmt.Errorf("error: no write permission for directory '%s'", outputFolder)
		}
	}
	return outputFolder, nil
}

func IsFlag(arg string) bool {
	if arg == "-l" || arg == "-r" || arg == "-p" || arg == "-v" {
		return true
	}
	return false
}

func getURL(argList []string) (string, error) {
	url := ""
	for i := 0; i < len(argList); i++ {
		if IsFlag(argList[i]) {
			if argList[i] == "-l" || argList[i] == "-p" {
				i++
			}
			continue
		}
		if url == "" {
			url = argList[i]
		} else {
			return "", errors.New("error: too many URLs")
		}
	}
	if url == "" {
		return url, errors.New("error: missing URL")
	}
	return url, nil
}

func ErrorExit(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func isVerbose(argList []string) bool {
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-v" {
			return true
		}
	}
	return false
}

func parser(argList []string) (*SpiderOptions, error) {
	recurseLevel, err := getRecurseLevel(argList)
	if err != nil {
		return nil, err
	}
	url, err := getURL(argList)
	if err != nil {
		return nil, err
	}
	outputFolder, err := getOutputFolder(argList)
	if err != nil {
		return nil, err
	}
	verbose := isVerbose(argList)
	ret := SpiderOptions {
		OutputFolder: outputFolder,
		BaseUrl:      url,
		Verbose:      verbose,
		RecurseLevel: recurseLevel,
		WorkerCount:  42,
		Timeout:      120 * time.Second,
	}
	return &ret, nil
}

func hasImageSuffix(link string) bool {
	if strings.HasSuffix(link, ".jpg") || strings.HasSuffix(link, ".jpeg") ||
		strings.HasSuffix(link, ".png") || strings.HasSuffix(link, ".bmp") ||
		strings.HasSuffix(link, ".gif") {
		return true
	}
	return false
}

func isOctetStream(resp *http.Response) bool {
	if resp.Header.Get("Content-Type") == "application/octet-stream" {
		return true
	}
	return false
}

func parseFile(body []byte, target string) bool{
	if bytes.Contains(body, []byte("flag")) {
			return true 
	}
	return false
}

// TODO: might change the way image is handled by using the header with a switch on Content-Type
func downloadData(resp *http.Response, client *http.Client, opts *SpiderOptions, pageURL string, outputMutex *sync.Mutex) {
	defer resp.Body.Close()
	if hasImageSuffix(pageURL) || isOctetStream(resp) {
		// downloadFile(resp, client, pageURL, outputFolder, verbose, outputMutex)
		target := "flag"
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		if parseFile(body, target) {
			printMsg(fmt.Sprintf("Found file @ %s containing [ %s ]\n", pageURL, target), opts.Verbose, outputMutex)
			downloadFile(body, pageURL, opts.OutputFolder, opts.Verbose, outputMutex)
		}
	}
}

func extractLinks(body io.Reader, baseURL string) []string {
	var links []string
	tokenizer := html.NewTokenizer(body)
	base, _ := url.Parse(baseURL)

	for {
		token := tokenizer.Next()
		switch token {
		case html.ErrorToken:
			return links
		case html.StartTagToken:
			tagName, _ := tokenizer.TagName()
			if string(tagName) == "a" || string(tagName) == "img" {
				for {
					attrName, attrVal, moreAttr := tokenizer.TagAttr()
					if string(attrName) == "href" || string(attrName) == "src" {
						link, err := url.Parse(string(attrVal))
						if link.Path == "../" || link.Path == "./" {
							continue
						} else if err == nil {
							absoluteURL := base.ResolveReference(link).String()
							links = append(links, absoluteURL)
						}
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}
}

func downloadFile(body []byte, fileURL string, outputFolder string, verbose bool, outputMutex *sync.Mutex) {
	fileName := path.Base(fileURL)
	filePath := path.Join(outputFolder, fileName)

	base_filePath := strings.TrimSuffix(fileName, path.Ext(fileName))
 
	dupCount := 0

	for {
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
					break
			}
			dupCount++
			filePath = path.Join(outputFolder, fmt.Sprintf("%s-dup%d%s", base_filePath, dupCount, path.Ext(fileName)))
	}
	out, err := os.Create(filePath)
	if err != nil {
		printMsg(fmt.Sprintf("Error creating file: %s\n", err), verbose, outputMutex)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, bytes.NewReader(body))
	if err != nil {
		printMsg(fmt.Sprintf("Error saving file: %s\n", err), verbose, outputMutex)
	}
}

func processCurrentPage(ctx context.Context, client *http.Client, opts *SpiderOptions, pageURL string, resp *http.Response,
	outputMutex *sync.Mutex,
) {
	// printMsg(fmt.Sprintf("Downloading data from '%s'\n", pageURL), opts.Verbose, outputMutex)
	downloadData(resp, client, opts, pageURL, outputMutex)
}

func processURL(ctx context.Context, client *http.Client, opts *SpiderOptions,pageURL string, depth int, 
	queue chan struct {
		url   string
		depth int
	},
	startDomain *url.URL, mutex *sync.Mutex,
	outputMutex *sync.Mutex,
	visited *sync.Map, activeWorkers *int32,
) {
	defer atomic.AddInt32(activeWorkers, -1)
	if pageURL == "" {
		return
	}
	resp, err := client.Get(pageURL)
	if err != nil {
		printMsg(fmt.Sprintf("Error fetching page: %s\n", err), opts.Verbose, outputMutex)
		return
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") == "text/html" {
		links := extractLinks(resp.Body, pageURL)
		for _, link := range links {
			linkURL, err := url.Parse(link)
			if err != nil || linkURL.Host != startDomain.Host || depth+1 > opts.RecurseLevel {
				continue
			}
			if _, seen := visited.LoadOrStore(link, struct{}{}); !seen {
				select {
				case <-ctx.Done():
					return
				case queue <- struct {
					url   string
					depth int
				}{link, depth + 1}:
				}
			}
			atomic.AddInt32(activeWorkers, 1)
		}
	}
	processCurrentPage(ctx, client, opts, pageURL, resp, outputMutex)
}

func workerFunc(workerId int, ctx context.Context, client *http.Client, opts *SpiderOptions,
	queue chan struct {
		url   string
		depth int
	},
	startDomain *url.URL, mutex *sync.Mutex,
	outputMutex *sync.Mutex,
	visited *sync.Map, activeWorkers *int32,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-queue:
			processURL(ctx, client, opts, job.url, job.depth, queue,
				startDomain, mutex, outputMutex, visited, activeWorkers)
		}
	}
}

func printStats(activeWorkers int32, outputMutex *sync.Mutex) {
	outputMutex.Lock()
	defer outputMutex.Unlock()
	fmt.Print("\033[F\033[K")
	fmt.Print("\033[F\033[K")
	fmt.Print("\033[F\033[K")
	fmt.Printf("stats:\n channel status: remaining links %d\n", activeWorkers)
	fmt.Print("\033[3B")
}

func printMsg(msg string, verbose bool, outputMutex *sync.Mutex) {
	outputMutex.Lock()
	defer outputMutex.Unlock()
	if verbose {
		fmt.Print("\033[4F\033[K")
		fmt.Printf("%s", msg)
		fmt.Print("\033[3B")
	} else {
		fmt.Print("\033[F\033[K")
		fmt.Printf("%s", msg)
	}
}

func crawl(opts *SpiderOptions) {
	var visited sync.Map
	var wg sync.WaitGroup
	mutex := &sync.Mutex{}
	outputMutex := &sync.Mutex{}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	transport := &http.Transport{
			MaxIdleConns:        1000, 
			MaxIdleConnsPerHost: 500, 
			MaxConnsPerHost:     500,
			IdleConnTimeout:     5 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
	startDomain, _ := url.Parse(opts.BaseUrl)
	queue := make(chan struct {
		url   string
		depth int
	}, 20000)

	var activeWorkers int32 = 0

	atomic.AddInt32(&activeWorkers, 1)
	queue <- struct {
		url   string
		depth int
	}{opts.BaseUrl, 0}
	concurrentWorkers := 100
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go workerFunc(i, ctx, client, opts, queue, startDomain, mutex,
			outputMutex, &visited, &activeWorkers, &wg)
	}

	for {
		if atomic.LoadInt32(&activeWorkers) == 0 {
			cancel()
			break
		}
		time.Sleep(100 * time.Millisecond)
		if opts.Verbose {
			printStats(atomic.LoadInt32(&activeWorkers), outputMutex)
		}
	}
	select {
	case <-ctx.Done():
		endHandler(atomic.LoadInt32(&activeWorkers))
	default:
		fmt.Println("Non-Standard Error")
	}
	wg.Wait()
}

func endHandler(activeWorkers int32) {
	if activeWorkers == 0 {
		fmt.Println("All downloads completed successfully")
	} else {
		fmt.Println("HaySpider timed out")
	}
}
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./hsp [-rlp] URL")
		os.Exit(1)
	}
	opts, err := parser(os.Args[1:])
	if err != nil {
		ErrorExit(err)
	}
	fmt.Println("HaySpider is starting: depth-level", opts.RecurseLevel, "url", opts.BaseUrl, "outputFolder", opts.OutputFolder)
	if opts.Verbose {
		fmt.Printf("Latest Message:\n\nstats:\n active link jobs 0 || active downloads job 0\n channel status: link_process 0 || download_process 0\n")
	} else {
		fmt.Printf("Latest Message:\n\n")
	}
	crawl(opts)

	os.Exit(0)
}
