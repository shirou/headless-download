package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/raff/godet"
)

func findNewSrc(root string, attrs []string) string {
	fmt.Println(attrs)
	for i, attr := range attrs {
		if attr != "href" && attr != "src" {
			continue
		}
		url, err := url.Parse(attrs[i+1])
		if err != nil {
			log.Fatalf("url parse error: %v", attrs[i+1])
		}

		fname := filepath.Base(url.Path)
		return filepath.Join(root, fname)
	}

	return ""
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

func replace(remote *godet.RemoteDebugger, root, query string) error {
	fmt.Println(root, query)
	res, err := remote.QuerySelectorAll(1, query)
	if err != nil {
		return err
	}
	if res == nil {
		log.Fatalf("not err")
		return nil
	}
	for _, i := range res["nodeIds"].([]interface{}) {
		nodeId, ok := i.(float64)
		if !ok {
			continue
		}
		node, err := GetAttributes(remote, int(nodeId))
		if err != nil {
			return err
		}
		f := findNewSrc(root, node)

		if err := remote.SetAttributeValue(int(nodeId), "src", f); err != nil {
			return err
		}
	}
	return nil
}

func writeBody(remote *godet.RemoteDebugger, filename string) error {
	body, err := remote.GetOuterHTML(1)
	if err != nil {
		return err
	}
	if body == "" {
		log.Fatalf("GetOUterHtmlfailed: %v", err)
	}
	return ioutil.WriteFile(filename, []byte(body), 0644)
}

func main() {

	remote, _ := godet.Connect("localhost:9222", true)
	defer remote.Close()

	var wg sync.WaitGroup
	var urls []string

	remote.CallbackEvent("Network.requestWillBeSent", func(params godet.Params) {
		fmt.Println("requestWillBeSent",
			params["type"],
			params["documentURL"],
			params["request"].(map[string]interface{})["url"])
		url, ok := params["documentURL"].(string)
		if ok {
			urls = append(urls, url)
		}
	})

	remote.CallbackEvent("Page.loadEventFired", func(params godet.Params) {
		defer wg.Done()

		_, err := remote.GetDocument() // なぜか必要
		if err != nil {
			log.Fatalf("GetDocumentFailed: %v", err)
		}

		if err := replace(remote, "contents", "[src]"); err != nil {
			log.Fatalf("replace : %v", err)
		}
		if err := replace(remote, "contents", "[href]"); err != nil {
			log.Fatalf("replace : %v", err)
		}

		if err := writeBody(remote, "index.html"); err != nil {
			log.Fatalf("GetOuterHTML : %v", err)
		}
	})

	wg.Add(1)
	_, _ = remote.Navigate("https://github.com")
	remote.NetworkEvents(true)
	remote.PageEvents(true)

	//	remote.RequestWillBeSent(true)

	wg.Wait()

	remote.Close()
}
