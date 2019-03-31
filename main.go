package main

import (
	"github.com/gocolly/colly"

	//"github.com/gocolly/colly/debug"
	"comp4321/concurrentMap"
	Indexer "comp4321/indexer"
	"comp4321/tokenizer"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type pageMap struct {
	id       uint64
	children concurrentMap.ConcurrentMap
	parent   concurrentMap.ConcurrentMap
}

var requestID string

func main() {
	var wg = &sync.WaitGroup{}
	wd, _ := os.Getwd()

	// rootPage := "https://www.cse.ust.hk"
	rootPage := "https://apartemen.win/comp4321/page1.html"

	tokenizer.LoadStopWords()

	documentIndexer := &Indexer.MappingIndexer{}
	docErr := documentIndexer.Initialize(wd + "/db/documentIndex")
	if docErr != nil {
		fmt.Printf("error when initializing document indexer: %s\n", docErr)
	}
	defer documentIndexer.Backup()
	defer documentIndexer.Release()

	wordIndexer := &Indexer.MappingIndexer{}
	wordErr := wordIndexer.Initialize(wd + "/db/wordIndex")
	if wordErr != nil {
		fmt.Printf("error when initializing word indexer: %s\n", wordErr)
	}
	defer wordIndexer.Backup()
	defer wordIndexer.Release()

	pagePropertiesIndexer := &Indexer.PagePropetiesIndexer{}
	pagePropertiesErr := pagePropertiesIndexer.Initialize(wd + "/db/pagePropertiesIndex")
	if pagePropertiesErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", pagePropertiesErr)
	}
	defer pagePropertiesIndexer.Backup()
	defer pagePropertiesIndexer.Release()

	titleInvertedIndexer := &Indexer.InvertedFileIndexer{}
	titleInvertedErr := titleInvertedIndexer.Initialize(wd + "/db/titleInvertedIndex")
	if titleInvertedErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", titleInvertedErr)
	}
	defer titleInvertedIndexer.Backup()
	defer titleInvertedIndexer.Release()

	contentInvertedIndexer := &Indexer.InvertedFileIndexer{}
	contentInvertedErr := contentInvertedIndexer.Initialize(wd + "/db/contentInvertedIndex")
	if contentInvertedErr != nil {
		fmt.Printf("error when initializing page properties: %s\n", contentInvertedErr)
	}
	defer contentInvertedIndexer.Backup()
	defer contentInvertedIndexer.Release()

	documentWordForwardIndexer := &Indexer.DocumentWordForwardIndexer{}
	documentWordForwardIndexerErr := documentWordForwardIndexer.Initialize(wd + "/db/documentWordForwardIndex")
	if documentWordForwardIndexerErr != nil {
		fmt.Printf("error when initializing document -> word forward Indexer: %s\n", documentWordForwardIndexerErr)
	}
	defer documentWordForwardIndexer.Backup()
	defer documentWordForwardIndexer.Release()

	parentChildDocumentForwardIndexer := &Indexer.ForwardIndexer{}
	parentChildDocumentForwardIndexerErr := parentChildDocumentForwardIndexer.Initialize(wd + "/db/parentChildDocumentForwardIndex")
	if parentChildDocumentForwardIndexerErr != nil {
		fmt.Printf("error when initializing parentDocument -> childDocument forward Indexer: %s\n", parentChildDocumentForwardIndexerErr)
	}
	defer parentChildDocumentForwardIndexer.Backup()
	defer parentChildDocumentForwardIndexer.Release()

	childParentDocumentForwardIndexer := &Indexer.ForwardIndexer{}
	childParentDocumentForwardIndexerErr := childParentDocumentForwardIndexer.Initialize(wd + "/db/childParentDocumentForwardIndex")
	if childParentDocumentForwardIndexerErr != nil {
		fmt.Printf("error when initializing childDocument -> parentDocument forward Indexer: %s\n", childParentDocumentForwardIndexerErr)
	}
	defer childParentDocumentForwardIndexer.Backup()
	defer childParentDocumentForwardIndexer.Release()

	pages := make([]pageMap, 0)
	maxDepth := 3
	crawler := colly.NewCollector(
		colly.MaxDepth(maxDepth),
		// colly.Debugger(&debug.LogDebugger{}),
		colly.Async(true),
	)

	// Limit the maximum parallelism to 2
	// This is necessary if the goroutines are dynamically
	// created to control the limit of simultaneous requests.
	//
	// Parallelism can be controlled also by spawning fixed
	// number of go routines.
	crawler.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2})

	crawler.OnResponse(func(r *colly.Response) {
		fmt.Println("Visited", r.Request.URL)
		fmt.Println("")
	})

	crawler.OnHTML("html", func(e *colly.HTMLElement) {

		title := e.ChildText("title")
		url := e.Request.URL.String()

		size, _ := strconv.Atoi(e.Response.Headers.Get("Content-Length"))

		if size == 0 {
			size = len(e.Text)
		}

		date := e.Response.Headers.Get("Last-Modified")
		dateTime := time.Time{}
		if len(date) != 0 {
			dateTime, _ = time.Parse(time.RFC1123, date)
		} else {
			dateTime = time.Now()
		}

		// Store Document id and properties

		id, err := documentIndexer.GetValueFromKey(url)
		if err != nil {
			id, _ = documentIndexer.AddKeyToIndex(url)
		}
		pagePropertiesIndexer.AddKeyToPageProperties(id, Indexer.CreatePage(id, title, url, size, dateTime))

		content := tokenizer.Tokenize(e.ChildText("body"))

		processedTitle := tokenizer.Tokenize(title)

		titleWordList := make(map[uint64]*Indexer.InvertedFile)
		for i, v := range processedTitle {
			// Add Word to id index
			wordID, err := wordIndexer.GetValueFromKey(v)
			if err != nil {
				wordID, _ = wordIndexer.AddKeyToIndex(v)
			}

			invFile, contain := titleWordList[wordID]
			if contain {
				invFile.AddWordPositions(uint64(i))
			} else {
				titleWordList[wordID] = Indexer.CreateInvertedFile(id)
				titleWordList[wordID].AddWordPositions(uint64(i))
			}
		}

		for k, v := range titleWordList {
			titleInvertedIndexer.AddKeyToIndexOrUpdate(k, *v)
		}

		// Check for duplicate words in the document
		contentWordList := make(map[uint64]*Indexer.InvertedFile)
		contentWordCounter := make(map[uint64]uint64)
		for i, v := range content {
			// Add Word to id index
			wordID, err := wordIndexer.GetValueFromKey(v)
			if err != nil {
				wordID, _ = wordIndexer.AddKeyToIndex(v)
			}

			invFile, contain := contentWordList[wordID]
			if contain {
				invFile.AddWordPositions(uint64(i))
			} else {
				contentWordList[wordID] = Indexer.CreateInvertedFile(id)
				contentWordList[wordID].AddWordPositions(uint64(i))
			}
			if _, contain = contentWordCounter[wordID]; contain {
				contentWordCounter[wordID]++
			} else {
				contentWordCounter[wordID] = 1
			}

		}

		for k, v := range contentWordList {
			contentInvertedIndexer.AddKeyToIndexOrUpdate(k, *v)
		}

		// Get Unique Number of words in the map
		wordFrequencySlice := make([]Indexer.WordFrequency, 0)
		for k, v := range contentWordCounter {
			wordFrequencySlice = append(wordFrequencySlice, Indexer.CreateWordFrequency(k, v))
		}
		documentWordForwardIndexer.AddWordFrequencyListToKey(id, wordFrequencySlice)

		tempMap := pageMap{}
		tempMap.id = id
		tempMap.children = concurrentMap.ConcurrentMap{}
		tempMap.children.Init()
		// temp.date_modified = e.Response.Headers.Get("Last-Modified")
		links := e.ChildAttrs("a[href]", "href")
		for _, url := range links {
			url = e.Request.AbsoluteURL(url)
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				resp, err := http.Get(url)
				if err != nil || resp.StatusCode != 200 {
					return
				}
				defer resp.Body.Close()

				childID, err := documentIndexer.GetValueFromKey(url)
				if err != nil {
					childID, _ = documentIndexer.AddKeyToIndex(url)
				}
				if _, ok := tempMap.children.Get(childID); !ok {
					tempMap.children.Set(childID, nil)
				}

				p := Indexer.CreatePage(childID, "", url, 0, time.Now())
				if _, err := pagePropertiesIndexer.GetPagePropertiesFromKey(childID); err != nil {
					pagePropertiesIndexer.AddKeyToPageProperties(childID, p)
				}

				e.Request.Visit(url)
			}(url)
		}
		wg.Wait()
		pages = append(pages, tempMap)
		parentChildDocumentForwardIndexer.AddIdListToKey(id, tempMap.children.ConvertToSliceOfKeys())
	})

	crawler.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	crawler.Visit(rootPage)

	crawler.Wait()
	// After finished, iterate over all pages to get child->parent relation
	for _, page := range pages {
		page.parent.Init()
		for _, v := range pages {
			if _, contains := v.children.Get(page.id); contains {
				page.parent.Set(v.id, nil)
			}
		}
		childParentDocumentForwardIndexer.AddIdListToKey(page.id, page.parent.ConvertToSliceOfKeys())
	}

	documentIndexer.Iterate()
	contentInvertedIndexer.Iterate()
	wordIndexer.Iterate()
	pagePropertiesIndexer.Iterate()
	documentWordForwardIndexer.Iterate()
	parentChildDocumentForwardIndexer.Iterate()
	childParentDocumentForwardIndexer.Iterate()

	// i, _ := documentIndexer.GetValueFromKey("https://apartemen.win/comp4321/page1.html")
	// l, _ := documentWordForwardIndexer.GetIdListFromKey(i)

	// err := contentInvertedIndexer.DeleteInvertedFileFromWordListAndPage(l, i)
	// fmt.Println(err)
}
