package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/raff/godet"
)

func findNewSrc(root string, attrs []string) (string, string, string) {
	for i, attr := range attrs {
		if attr != "href" && attr != "src" {
			continue
		}
		n := attrs[i+1]

		// skip if data attribute
		if strings.Contains(n, "data") {
			return attr, n, n
		}
		url, err := url.Parse(n)
		if err != nil {
			log.Fatalf("url parse error: %v", n)
		}

		fname := filepath.Base(url.Path)
		return attr, n, filepath.Join(root, fname)
	}

	return "", "", ""
}

func GetTitle(remote *godet.RemoteDebugger) (string, error) {
	res, err := remote.GetDocument()
	if err != nil {
		return "", err
	}
	title := "" // TODO
	fmt.Println(res)
	return title, nil
}

var filter = []string{
	".js",
	".css",
	".png",
	".svg",
	".jpg",
}

func willDownload(pathURL string) bool {
	url, err := url.Parse(pathURL)
	if err != nil {
		return false
	}
	for _, f := range filter {
		if strings.HasSuffix(url.Path, f) {
			return true
		}
	}
	return false
}

func GetAttributes(remote *godet.RemoteDebugger, nodeId int) ([]string, error) {
	params := godet.Params{
		"nodeId": nodeId,
	}

	res, err := remote.SendRequest("DOM.getAttributes", params)
	if err != nil {
		return []string{}, err
	}
	ret := make([]string, 0)
	for _, i := range res["attributes"].([]interface{}) {
		attr, ok := i.(string)
		if !ok {
			continue
		}
		ret = append(ret, attr)
	}

	return ret, nil
}

func replace(remote *godet.RemoteDebugger, root, query string) ([]string, error) {
	var orig []string

	res, err := remote.QuerySelectorAll(1, query)
	if err != nil {
		return nil, err
	}
	if res == nil {
		log.Fatalf("not err")
		return orig, nil
	}

	for _, i := range res["nodeIds"].([]interface{}) {
		nodeId, ok := i.(float64)
		if !ok {
			continue
		}
		node, err := GetAttributes(remote, int(nodeId))
		if err != nil {
			return nil, err
		}
		attr, original, f := findNewSrc(root, node)
		orig = append(orig, original)

		if err := remote.SetAttributeValue(int(nodeId), attr, f); err != nil {
			return nil, err
		}
	}
	return orig, nil
}

func download(reqURI, root string) error {
	if strings.HasPrefix(reqURI, "//") {
		reqURI = "http:" + reqURI
	}

	u, err := url.Parse(reqURI)
	if err != nil {
		return err
	}
	resp, err := http.Get(reqURI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//open a file for writing
	fname := filepath.Base(u.Path)
	fmt.Println(reqURI, filepath.Join(root, fname))
	file, err := os.Create(filepath.Join(root, fname))
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// add some utf-8 in the html entitiy to detect code
func setEncodeDetector(remote *godet.RemoteDebugger) error {
	res, err := remote.QuerySelector(1, "html")
	if err != nil {
		return err
	}
	nodeId, ok := res["nodeId"].(float64)
	if !ok {
		return fmt.Errorf("nodeId get failed")
	}

	if err := remote.SetAttributeValue(int(nodeId), "l", "美乳"); err != nil {
		return err
	}
	return nil
}

func writeBody(remote *godet.RemoteDebugger, filename string) error {
	setEncodeDetector(remote)

	body, err := remote.GetOuterHTML(1)
	if err != nil {
		return err
	}
	if body == "" {
		log.Fatalf("GetOUterHtmlfailed: %v", err)
	}
	return ioutil.WriteFile(filename, []byte(body), 0644)
}

func EnsureDirectory(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, 0755)
	}
}

func mergeURLs(orig, new []string) []string {
	appending := make([]string, 0)

	for _, n := range new {
		var found bool
		for _, o := range orig {
			if n == o {
				found = true
			}
		}
		if !found {
			appending = append(appending, n)
		}
	}

	return append(orig, appending...)
}

func main() {
	flag.Parse()
	URI := flag.Arg(0)

	u, err := url.Parse(URI)
	if err != nil {
		log.Fatalf("invalid url: %s", URI)
	}
	root := u.Host

	remote, _ := godet.Connect("localhost:9222", false)
	defer remote.Close()

	var wg sync.WaitGroup
	var urls []string

	remote.CallbackEvent("Network.requestWillBeSent", func(params godet.Params) {
		req, ok := params["request"].(map[string]interface{})
		if !ok {
			return
		}

		url, ok := req["url"].(string)
		if ok {
			urls = append(urls, url)
		}
	})

	remote.CallbackEvent("Page.loadEventFired", func(params godet.Params) {
		defer wg.Done()

		EnsureDirectory(root)

		_, err := remote.GetDocument() // なぜか必要
		if err != nil {
			log.Fatalf("GetDocumentFailed: %v", err)
		}
		{
			new, err := replace(remote, root, "[src]")
			if err != nil {
				log.Fatalf("replace : %v", err)
			}
			urls = mergeURLs(urls, new)
		}
		{
			new, err := replace(remote, root, "[href]")
			if err != nil {
				log.Fatalf("replace : %v", err)
			}
			urls = mergeURLs(urls, new)
		}

		for _, url := range urls {
			if err := download(url, root); err != nil {
				log.Printf("download failed: %s, %v", url, err)
			}
		}

		if err := writeBody(remote, root+".html"); err != nil {
			log.Fatalf("GetOuterHTML : %v", err)
		}
	})

	wg.Add(1)
	_, _ = remote.Navigate(URI)
	remote.NetworkEvents(true)
	remote.PageEvents(true)

	wg.Wait()

	remote.Close()
}
